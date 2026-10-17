package webuitest

import (
	"strings"

	"github.com/protolambda/chord/ct"
)

// Layout ids of webui's page shell, which every full page renders, so that
// fragments may target them: the main landmark, the alerts region, the
// dialogs region and the page header.
const (
	MainID    = "main"
	AlertsID  = "alerts"
	DialogsID = "dialogs"
	HeaderID  = "page-header"
)

// DocumentTargets returns the ids of the page layout that a fragment may
// reference without containing them ([MainID], [AlertsID], [DialogsID],
// [HeaderID]): the default document targets of [HXTargets] in [Rules].
// Each call returns a new slice.
func DocumentTargets() []string {
	return []string{MainID, AlertsID, DialogsID, HeaderID}
}

// RuleOption configures [Rules].
type RuleOption func(*ruleConfig)

type ruleConfig struct {
	csrf        string
	secrets     []string
	targets     []string
	bootstrapJS bool
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

// WithDocumentTargets replaces the ids that a fragment may reference
// without containing them ([HXTargets]); the default is
// [DocumentTargets].
func WithDocumentTargets(ids ...string) RuleOption {
	return func(c *ruleConfig) { c.targets = append([]string(nil), ids...) }
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
//   - CSRF: [PostFormsCarryCSRF] and [NoTokenLeaks] (see [WithCSRF]);
//   - htmx and components: [HXTargets] (with [DocumentTargets], see
//     [WithDocumentTargets]), [SSEWiring], [ModalDialogs] and
//     [NoBootstrapJSHooks] (see [WithBootstrapJS]);
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
		NoTokenLeaks(cfg.csrf),
		HXTargets(cfg.targets...),
		SSEWiring(),
		ModalDialogs(),
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
		rules = append(rules, SecretsHidden(cfg.secrets...), NotInURLs(cfg.secrets...))
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
