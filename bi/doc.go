// Package bi renders Bootstrap Icons (release [Version]) as chord nodes.
//
// An [Icon] is the name of an icon, and a node by itself:
//
//	text.P()(bi.Alarm, text.Text(" Reminder set"))
//
// renders <p><i class="bi bi-alarm" aria-hidden="true"></i> Reminder set</p>.
// Every icon of the release has a constant (Alarm, XLg, Num1Circle, ...), and
// [Lookup] validates a name that is only known at run time, e.g. one stored
// in settings.
//
// # Assets
//
// The icons are glyphs of the Bootstrap Icons web font: the page needs the
// icons stylesheet and its font, of release [Version]. The theme package
// ships both ([github.com/protolambda/webui/theme.IconsCSS] and
// [github.com/protolambda/webui/theme.IconsFont]) and includes them in its
// asset set unless the set is built without icons. Serve them from the
// page's own origin: a strict content security policy then only needs
// style-src 'self' and font-src 'self'. The stylesheet refers to the font by
// the relative URL fonts/bootstrap-icons.woff2, so the font must be served
// under the stylesheet's directory.
//
// # Accessibility
//
// An icon is decorative by default: it renders aria-hidden="true", so that
// assistive technology skips it. Use [Icon.Label] for an icon that carries
// meaning on its own; it renders role="img" and an aria-label instead. An
// icon-only control needs an accessible name on the control itself, e.g. an
// aria-label on the button, or visually hidden text next to the icon:
//
//	button.Button(button.Type(button.TypeButton), aria.Label("Delete note"))(bi.Trash)
//
// Use [Icon.With] to add classes (fs-4, text-danger, me-1, ...) or other
// attributes. The bi classes and the added classes render as a single class
// attribute.
//
// # Escaping
//
// The constants are safe literals. An Icon made from another string renders
// only when its name is lower-case ASCII letters, digits and hyphens; any
// other name (empty, with spaces, upper-case or with markup) fails the render
// with an error wrapping [ErrInvalidName], which does not repeat the name. A
// name thus cannot break out of the class attribute or add classes to the
// icon. Check a name only known at run time with [Lookup] before converting
// it, so that an unknown name is handled where it is read rather than as a
// render error. Labels are always escaped.
//
// # Generation
//
// list.go is generated from the stylesheet pinned by the theme package, by
// the command in bi/internal/gen. To update the icons, update the stylesheet
// and font in the theme package, point the default stylesheet path of the
// generator at the new file, and run go generate ./bi. The tests check that
// list.go is up to date, that the constants match the stylesheet, and that
// the font the stylesheet refers to exists.
package bi

//go:generate go run ./internal/gen
