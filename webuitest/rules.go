package webuitest

import (
	"strings"

	"github.com/protolambda/chord/ct"

	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/page"
)

// DocumentTargets returns the ids of the regions of page.Layout that a
// fragment may reference without containing them: [page.MainID],
// [page.AlertsID], [page.DialogsID], [page.HeaderID], and the default ids
// of the navbar and the sidebar, [page.NavbarID] and [page.SidebarID],
// which fragments update out of band (for example a badge). These are the
// default document targets of [HXTargets] in [Rules]; a navbar or sidebar
// with another id needs [WithDocumentTargets]. Each call returns a new
// slice.
func DocumentTargets() []string {
	return []string{page.MainID, page.AlertsID, page.DialogsID, page.HeaderID, page.NavbarID, page.SidebarID}
}

// RuleOption configures [Rules].
type RuleOption func(*ruleConfig)

type ruleConfig struct {
	csrf        string
	secrets     []string
	targets     []string
	dialogs     []string // shared dialogs
	bootstrapJS bool
	urlMethods  map[string]bool // of the page around fragments; nil: the zero hx2.Config
	without     map[string]bool
}

// WithCSRF makes the CSRF rules require token as the value of every CSRF
// field ([PostFormsCarryCSRF]) and look for leaks of it ([NoTokenLeaks]).
// Without it, they require a non-empty field and check its placement only.
func WithCSRF(token string) RuleOption {
	return func(c *ruleConfig) { c.csrf = token }
}

// WithSecrets adds [SecretsHidden] and [NotInURLs] for values. Empty values
// are a programming error: the rules panic when they are built.
func WithSecrets(values ...string) RuleOption {
	return func(c *ruleConfig) { c.secrets = append(c.secrets, values...) }
}

// WithHTMXConfig sets the htmx configuration of the pages that fragments
// are swapped into (page.Document.HTMX), for [NoTokenLeaks] and
// [NotInURLs]: a fragment has no htmx-config meta element of its own, and
// runs under the configuration of the page around it. The default is the
// zero hx2.Config, whose DELETE requests carry their parameters in the
// body; only hx2.Config.DeleteURLParams changes what the rules check, and
// hx2.Config{DeleteURLParams: true} checks fragments under htmx's own
// default (GET and DELETE parameters in the URL). Complete documents always use their own htmx-config meta element, or
// htmx's defaults without one. It is the rule counterpart of
// [Shell.HTMXConfig] and [PageHTMXConfig]. A config that does not encode
// (hx2.Config.JSON fails) is a programming error: WithHTMXConfig panics.
func WithHTMXConfig(c hx2.Config) RuleOption {
	methods := configURLMethods("webuitest.WithHTMXConfig", c)
	return func(rc *ruleConfig) { rc.urlMethods = methods }
}

// WithDocumentTargets replaces the ids that a fragment may reference
// without containing them ([HXTargets]); the default is
// [DocumentTargets].
func WithDocumentTargets(ids ...string) RuleOption {
	return func(c *ruleConfig) { c.targets = append([]string(nil), ids...) }
}

// WithSharedDialogs names dialogs that the document renders on every page
// for the openers that later fragments bring, such as a confirmation
// dialog in the dialogs region of page.Layout that the rows of a list
// loaded by htmx open: [ModalDialogs] requires no opener for them on a
// page. Every other dialog check still applies to them; in a fragment, no
// dialog needs an opener anyway. Repeated options add up.
func WithSharedDialogs(ids ...string) RuleOption {
	return func(c *ruleConfig) { c.dialogs = append(c.dialogs, ids...) }
}

// WithBootstrapJS drops [NoBootstrapJSHooks], for pages that load
// Bootstrap's script (theme.Options.BootstrapJS) and use its components.
func WithBootstrapJS() RuleOption {
	return func(c *ruleConfig) { c.bootstrapJS = true }
}

// Without drops rules by name: the part of the rule's String before the
// parenthesis, e.g. "imagesHaveAlt" or "postFormsCarryCSRF". Unknown names
// are ignored.
func Without(names ...string) RuleOption {
	return func(c *ruleConfig) {
		if c.without == nil {
			c.without = make(map[string]bool)
		}
		for _, n := range names {
			c.without[n] = true
		}
	}
}

// RuleName returns the name of a rule: its String up to the parenthesis,
// as [Without] takes it.
func RuleName(r ct.Rule) string {
	name, _, _ := strings.Cut(r.String(), "(")
	return name
}

// Rules returns the document rules that every webui page and fragment
// satisfies, for [ct.Subject.Valid], [PageContract] and
// [FragmentContract]:
//
//   - the structure rules of ct: [ct.UniqueIDs], [ct.LabelReferences],
//     [ct.ARIAReferences], [ct.LocalTargets], [ct.ImagesHaveAlt],
//     [ct.ButtonsHaveType], [ct.NoInlineHandlers] and [ct.NoRaw];
//   - the content security policy: [NoInlineScripts], [NoInlineStyles],
//     [NoHTMXScripting], [SafeURLs], [LocalResources] and
//     [SubresourceIntegrity];
//   - CSRF: [PostFormsCarryCSRF] and [NoTokenLeaks] (see [WithCSRF], and
//     [WithHTMXConfig] for fragments of pages with a non-default htmx
//     configuration);
//   - htmx and components: [HXTargets] (with [DocumentTargets], see
//     [WithDocumentTargets]), [HXPreserveIDs], [SSEWiring], [ModalDialogs]
//     (see [WithSharedDialogs]) and [NoBootstrapJSHooks] (see
//     [WithBootstrapJS]);
//   - accessibility and HTML quality: [IconsHidden], [ControlsNamed],
//     [NoNestedInteractive], [RectangularTables] and [ExternalLinksSafe];
//   - with [WithSecrets]: [SecretsHidden] and [NotInURLs].
//
// [Without] drops rules by name. Each call returns a new slice.
func Rules(opts ...RuleOption) []ct.Rule {
	cfg := ruleConfig{targets: DocumentTargets()}
	for _, o := range opts {
		o(&cfg)
	}
	rules := []ct.Rule{
		ct.UniqueIDs(),
		ct.LabelReferences(),
		ct.ARIAReferences(),
		ct.LocalTargets(),
		ct.ImagesHaveAlt(),
		ct.ButtonsHaveType(),
		ct.NoInlineHandlers(),
		ct.NoRaw(),
		NoInlineScripts(),
		NoInlineStyles(),
		NoHTMXScripting(),
		SafeURLs(),
		LocalResources(),
		SubresourceIntegrity(),
		PostFormsCarryCSRF(cfg.csrf),
		noTokenLeaks(cfg.csrf, cfg.urlMethods),
		HXTargets(cfg.targets...),
		HXPreserveIDs(),
		SSEWiring(),
		ModalDialogs(cfg.dialogs...),
		IconsHidden(),
		ControlsNamed(),
		NoNestedInteractive(),
		RectangularTables(),
		ExternalLinksSafe(),
	}
	if !cfg.bootstrapJS {
		rules = append(rules, NoBootstrapJSHooks())
	}
	if len(cfg.secrets) > 0 {
		rules = append(rules, SecretsHidden(cfg.secrets...), notInURLs(cfg.urlMethods, cfg.secrets...))
	}
	if len(cfg.without) == 0 {
		return rules
	}
	out := rules[:0]
	for _, r := range rules {
		if !cfg.without[RuleName(r)] {
			out = append(out, r)
		}
	}
	return out
}
