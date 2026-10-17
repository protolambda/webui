package ba_test

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/ba"
)

// bootstrapCSS is the pinned Bootstrap stylesheet of the theme package,
// relative to this package.
const bootstrapCSS = "../theme/files/bootstrap-5.3.8.min.css"

// classCase is one helper call and the class it must produce. The name is the
// call as written, e.g. "MT(3)"; the part before "(" names the helper.
type classCase struct {
	name string
	node attr.Node
	want string
}

// intHelper is a helper that takes an int, with its valid range.
type intHelper struct {
	name   string
	fn     func(int) attr.Node
	prefix string
	lo, hi int
}

// bpHelper is a helper that takes a Breakpoint, with the class pattern
// ("%s" is the breakpoint infix).
type bpHelper struct {
	name    string
	fn      func(ba.Breakpoint) attr.Node
	pattern string
}

var breakpoints = []ba.Breakpoint{ba.SM, ba.MD, ba.LG, ba.XL, ba.XXL}

func intHelpers() []intHelper {
	h := []intHelper{
		{"M", ba.M, "m-", 0, 5},
		{"MT", ba.MT, "mt-", 0, 5},
		{"MB", ba.MB, "mb-", 0, 5},
		{"MS", ba.MS, "ms-", 0, 5},
		{"ME", ba.ME, "me-", 0, 5},
		{"MX", ba.MX, "mx-", 0, 5},
		{"MY", ba.MY, "my-", 0, 5},
		{"P", ba.P, "p-", 0, 5},
		{"PT", ba.PT, "pt-", 0, 5},
		{"PB", ba.PB, "pb-", 0, 5},
		{"PS", ba.PS, "ps-", 0, 5},
		{"PE", ba.PE, "pe-", 0, 5},
		{"PX", ba.PX, "px-", 0, 5},
		{"PY", ba.PY, "py-", 0, 5},
		{"Gap", ba.Gap, "gap-", 0, 5},
		{"RowGap", ba.RowGap, "row-gap-", 0, 5},
		{"ColumnGap", ba.ColumnGap, "column-gap-", 0, 5},
		{"FS", ba.FS, "fs-", 1, 6},
		{"BorderWidth", ba.BorderWidth, "border-", 0, 5},
		{"RoundedSize", ba.RoundedSize, "rounded-", 0, 5},
		{"ColSM", ba.ColSM, "col-sm-", 1, 12},
		{"ColMD", ba.ColMD, "col-md-", 1, 12},
		{"ColLG", ba.ColLG, "col-lg-", 1, 12},
		{"ColXL", ba.ColXL, "col-xl-", 1, 12},
		{"ColXXL", ba.ColXXL, "col-xxl-", 1, 12},
		{"Offset", ba.Offset, "offset-", 1, 11},
		{"OffsetSM", ba.OffsetSM, "offset-sm-", 0, 11},
		{"OffsetMD", ba.OffsetMD, "offset-md-", 0, 11},
		{"OffsetLG", ba.OffsetLG, "offset-lg-", 0, 11},
		{"OffsetXL", ba.OffsetXL, "offset-xl-", 0, 11},
		{"OffsetXXL", ba.OffsetXXL, "offset-xxl-", 0, 11},
	}
	for _, bp := range breakpoints {
		h = append(h,
			intHelper{fmt.Sprintf("ColAt(%s)", bp), func(n int) attr.Node { return ba.ColAt(bp, n) }, "col-" + bp.String() + "-", 1, 12},
			intHelper{fmt.Sprintf("OffsetAt(%s)", bp), func(n int) attr.Node { return ba.OffsetAt(bp, n) }, "offset-" + bp.String() + "-", 0, 11},
		)
	}
	return h
}

func bpHelpers() []bpHelper {
	return []bpHelper{
		{"DNoneAt", ba.DNoneAt, "d-%s-none"},
		{"DInlineAt", ba.DInlineAt, "d-%s-inline"},
		{"DInlineBlockAt", ba.DInlineBlockAt, "d-%s-inline-block"},
		{"DBlockAt", ba.DBlockAt, "d-%s-block"},
		{"DGridAt", ba.DGridAt, "d-%s-grid"},
		{"DInlineGridAt", ba.DInlineGridAt, "d-%s-inline-grid"},
		{"DFlexAt", ba.DFlexAt, "d-%s-flex"},
		{"DInlineFlexAt", ba.DInlineFlexAt, "d-%s-inline-flex"},
		{"DTableAt", ba.DTableAt, "d-%s-table"},
		{"DTableRowAt", ba.DTableRowAt, "d-%s-table-row"},
		{"DTableCellAt", ba.DTableCellAt, "d-%s-table-cell"},
		{"FlexRowAt", ba.FlexRowAt, "flex-%s-row"},
		{"FlexColumnAt", ba.FlexColumnAt, "flex-%s-column"},
		{"ColAutoAt", ba.ColAutoAt, "col-%s-auto"},
		{"StickyTopAt", ba.StickyTopAt, "sticky-%s-top"},
		{"StickyBottomAt", ba.StickyBottomAt, "sticky-%s-bottom"},
		{"ColAt(n=1)", func(bp ba.Breakpoint) attr.Node { return ba.ColAt(bp, 1) }, "col-%s-1"},
		{"OffsetAt(n=0)", func(bp ba.Breakpoint) attr.Node { return ba.OffsetAt(bp, 0) }, "offset-%s-0"},
	}
}

func classCases() []classCase {
	cases := []classCase{
		{"FormControl()", ba.FormControl(), "form-control"},
		{"FormSelect()", ba.FormSelect(), "form-select"},
		{"FormCheckInput()", ba.FormCheckInput(), "form-check-input"},
		{"FW(bold)", ba.FW("bold"), "fw-bold"},
		{"FW(bolder)", ba.FW("bolder"), "fw-bolder"},
		{"FW(semibold)", ba.FW("semibold"), "fw-semibold"},
		{"FW(medium)", ba.FW("medium"), "fw-medium"},
		{"FW(normal)", ba.FW("normal"), "fw-normal"},
		{"FW(light)", ba.FW("light"), "fw-light"},
		{"FW(lighter)", ba.FW("lighter"), "fw-lighter"},
		{"LH(1)", ba.LH("1"), "lh-1"},
		{"LH(sm)", ba.LH("sm"), "lh-sm"},
		{"LH(base)", ba.LH("base"), "lh-base"},
		{"LH(lg)", ba.LH("lg"), "lh-lg"},
		{"BtnSM", ba.BtnSM(), "btn-sm"},
		{"BtnLG", ba.BtnLG(), "btn-lg"},
		{"BgPrimary", ba.BgPrimary(), "bg-primary"},
		{"BgSecondary", ba.BgSecondary(), "bg-secondary"},
		{"BgSuccess", ba.BgSuccess(), "bg-success"},
		{"BgDanger", ba.BgDanger(), "bg-danger"},
		{"BgWarning", ba.BgWarning(), "bg-warning"},
		{"BgInfo", ba.BgInfo(), "bg-info"},
		{"BgLight", ba.BgLight(), "bg-light"},
		{"BgDark", ba.BgDark(), "bg-dark"},
		{"BgBlack", ba.BgBlack(), "bg-black"},
		{"BgWhite", ba.BgWhite(), "bg-white"},
		{"BgBody", ba.BgBody(), "bg-body"},
		{"BgBodySecondary", ba.BgBodySecondary(), "bg-body-secondary"},
		{"BgBodyTertiary", ba.BgBodyTertiary(), "bg-body-tertiary"},
		{"BgTransparent", ba.BgTransparent(), "bg-transparent"},
		{"BgPrimarySubtle", ba.BgPrimarySubtle(), "bg-primary-subtle"},
		{"BgSecondarySubtle", ba.BgSecondarySubtle(), "bg-secondary-subtle"},
		{"BgSuccessSubtle", ba.BgSuccessSubtle(), "bg-success-subtle"},
		{"BgDangerSubtle", ba.BgDangerSubtle(), "bg-danger-subtle"},
		{"BgWarningSubtle", ba.BgWarningSubtle(), "bg-warning-subtle"},
		{"BgInfoSubtle", ba.BgInfoSubtle(), "bg-info-subtle"},
		{"BgLightSubtle", ba.BgLightSubtle(), "bg-light-subtle"},
		{"BgDarkSubtle", ba.BgDarkSubtle(), "bg-dark-subtle"},
		{"TextBgPrimary", ba.TextBgPrimary(), "text-bg-primary"},
		{"TextBgSecondary", ba.TextBgSecondary(), "text-bg-secondary"},
		{"TextBgSuccess", ba.TextBgSuccess(), "text-bg-success"},
		{"TextBgDanger", ba.TextBgDanger(), "text-bg-danger"},
		{"TextBgWarning", ba.TextBgWarning(), "text-bg-warning"},
		{"TextBgInfo", ba.TextBgInfo(), "text-bg-info"},
		{"TextBgLight", ba.TextBgLight(), "text-bg-light"},
		{"TextBgDark", ba.TextBgDark(), "text-bg-dark"},
		{"OverflowAuto", ba.OverflowAuto(), "overflow-auto"},
		{"OverflowHidden", ba.OverflowHidden(), "overflow-hidden"},
		{"OverflowVisible", ba.OverflowVisible(), "overflow-visible"},
		{"OverflowScroll", ba.OverflowScroll(), "overflow-scroll"},
		{"DNone", ba.DNone(), "d-none"},
		{"DInline", ba.DInline(), "d-inline"},
		{"DInlineBlock", ba.DInlineBlock(), "d-inline-block"},
		{"DBlock", ba.DBlock(), "d-block"},
		{"DGrid", ba.DGrid(), "d-grid"},
		{"DInlineGrid", ba.DInlineGrid(), "d-inline-grid"},
		{"DFlex", ba.DFlex(), "d-flex"},
		{"DInlineFlex", ba.DInlineFlex(), "d-inline-flex"},
		{"DTable", ba.DTable(), "d-table"},
		{"DTableRow", ba.DTableRow(), "d-table-row"},
		{"DTableCell", ba.DTableCell(), "d-table-cell"},
		{"DPrintNone", ba.DPrintNone(), "d-print-none"},
		{"DPrintBlock", ba.DPrintBlock(), "d-print-block"},
		{"W25", ba.W25(), "w-25"},
		{"W50", ba.W50(), "w-50"},
		{"W75", ba.W75(), "w-75"},
		{"W100", ba.W100(), "w-100"},
		{"WAuto", ba.WAuto(), "w-auto"},
		{"H25", ba.H25(), "h-25"},
		{"H50", ba.H50(), "h-50"},
		{"H75", ba.H75(), "h-75"},
		{"H100", ba.H100(), "h-100"},
		{"HAuto", ba.HAuto(), "h-auto"},
		{"MW100", ba.MW100(), "mw-100"},
		{"MH100", ba.MH100(), "mh-100"},
		{"VWMin100", ba.VWMin100(), "min-vw-100"},
		{"VHMin100", ba.VHMin100(), "min-vh-100"},
		{"MAuto", ba.MAuto(), "m-auto"},
		{"MTAuto", ba.MTAuto(), "mt-auto"},
		{"MBAuto", ba.MBAuto(), "mb-auto"},
		{"MSAuto", ba.MSAuto(), "ms-auto"},
		{"MEAuto", ba.MEAuto(), "me-auto"},
		{"MXAuto", ba.MXAuto(), "mx-auto"},
		{"MYAuto", ba.MYAuto(), "my-auto"},
		{"PositionStatic", ba.PositionStatic(), "position-static"},
		{"PositionRelative", ba.PositionRelative(), "position-relative"},
		{"PositionAbsolute", ba.PositionAbsolute(), "position-absolute"},
		{"PositionFixed", ba.PositionFixed(), "position-fixed"},
		{"PositionSticky", ba.PositionSticky(), "position-sticky"},
		{"Top0", ba.Top0(), "top-0"},
		{"Top50", ba.Top50(), "top-50"},
		{"Top100", ba.Top100(), "top-100"},
		{"Bottom0", ba.Bottom0(), "bottom-0"},
		{"Bottom50", ba.Bottom50(), "bottom-50"},
		{"Bottom100", ba.Bottom100(), "bottom-100"},
		{"Start0", ba.Start0(), "start-0"},
		{"Start50", ba.Start50(), "start-50"},
		{"Start100", ba.Start100(), "start-100"},
		{"End0", ba.End0(), "end-0"},
		{"End50", ba.End50(), "end-50"},
		{"End100", ba.End100(), "end-100"},
		{"TranslateMiddle", ba.TranslateMiddle(), "translate-middle"},
		{"TranslateMiddleX", ba.TranslateMiddleX(), "translate-middle-x"},
		{"TranslateMiddleY", ba.TranslateMiddleY(), "translate-middle-y"},
		{"FixedTop", ba.FixedTop(), "fixed-top"},
		{"FixedBottom", ba.FixedBottom(), "fixed-bottom"},
		{"StickyTop", ba.StickyTop(), "sticky-top"},
		{"StickyBottom", ba.StickyBottom(), "sticky-bottom"},
		{"Visible", ba.Visible(), "visible"},
		{"Invisible", ba.Invisible(), "invisible"},
		{"VisuallyHidden", ba.VisuallyHidden(), "visually-hidden"},
		{"VisuallyHiddenFocusable", ba.VisuallyHiddenFocusable(), "visually-hidden-focusable"},
		{"TextStart", ba.TextStart(), "text-start"},
		{"TextCenter", ba.TextCenter(), "text-center"},
		{"TextEnd", ba.TextEnd(), "text-end"},
		{"TextWrap", ba.TextWrap(), "text-wrap"},
		{"TextNowrap", ba.TextNowrap(), "text-nowrap"},
		{"TextBreak", ba.TextBreak(), "text-break"},
		{"TextTruncate", ba.TextTruncate(), "text-truncate"},
		{"TextLowercase", ba.TextLowercase(), "text-lowercase"},
		{"TextUppercase", ba.TextUppercase(), "text-uppercase"},
		{"TextCapitalize", ba.TextCapitalize(), "text-capitalize"},
		{"TextDecorationNone", ba.TextDecorationNone(), "text-decoration-none"},
		{"TextDecorationUnderline", ba.TextDecorationUnderline(), "text-decoration-underline"},
		{"TextDecorationLineThrough", ba.TextDecorationLineThrough(), "text-decoration-line-through"},
		{"TextMuted", ba.TextMuted(), "text-body-secondary"},
		{"TextPrimary", ba.TextPrimary(), "text-primary"},
		{"TextSecondary", ba.TextSecondary(), "text-secondary"},
		{"TextSuccess", ba.TextSuccess(), "text-success"},
		{"TextDanger", ba.TextDanger(), "text-danger"},
		{"TextWarning", ba.TextWarning(), "text-warning"},
		{"TextInfo", ba.TextInfo(), "text-info"},
		{"TextLight", ba.TextLight(), "text-light"},
		{"TextDark", ba.TextDark(), "text-dark"},
		{"TextBlack", ba.TextBlack(), "text-black"},
		{"TextWhite", ba.TextWhite(), "text-white"},
		{"TextBody", ba.TextBody(), "text-body"},
		{"TextBodySecondary", ba.TextBodySecondary(), "text-body-secondary"},
		{"TextBodyTertiary", ba.TextBodyTertiary(), "text-body-tertiary"},
		{"TextBodyEmphasis", ba.TextBodyEmphasis(), "text-body-emphasis"},
		{"TextReset", ba.TextReset(), "text-reset"},
		{"TextPrimaryEmphasis", ba.TextPrimaryEmphasis(), "text-primary-emphasis"},
		{"TextSecondaryEmphasis", ba.TextSecondaryEmphasis(), "text-secondary-emphasis"},
		{"TextSuccessEmphasis", ba.TextSuccessEmphasis(), "text-success-emphasis"},
		{"TextDangerEmphasis", ba.TextDangerEmphasis(), "text-danger-emphasis"},
		{"TextWarningEmphasis", ba.TextWarningEmphasis(), "text-warning-emphasis"},
		{"TextInfoEmphasis", ba.TextInfoEmphasis(), "text-info-emphasis"},
		{"TextLightEmphasis", ba.TextLightEmphasis(), "text-light-emphasis"},
		{"TextDarkEmphasis", ba.TextDarkEmphasis(), "text-dark-emphasis"},
		{"LinkPrimary", ba.LinkPrimary(), "link-primary"},
		{"LinkSecondary", ba.LinkSecondary(), "link-secondary"},
		{"LinkSuccess", ba.LinkSuccess(), "link-success"},
		{"LinkDanger", ba.LinkDanger(), "link-danger"},
		{"LinkWarning", ba.LinkWarning(), "link-warning"},
		{"LinkInfo", ba.LinkInfo(), "link-info"},
		{"LinkLight", ba.LinkLight(), "link-light"},
		{"LinkDark", ba.LinkDark(), "link-dark"},
		{"LinkBodyEmphasis", ba.LinkBodyEmphasis(), "link-body-emphasis"},
		{"FWBold", ba.FWBold(), "fw-bold"},
		{"FWSemibold", ba.FWSemibold(), "fw-semibold"},
		{"FWMedium", ba.FWMedium(), "fw-medium"},
		{"FWNormal", ba.FWNormal(), "fw-normal"},
		{"FWLight", ba.FWLight(), "fw-light"},
		{"FSItalic", ba.FSItalic(), "fst-italic"},
		{"FSNormal", ba.FSNormal(), "fst-normal"},
		{"FontMonospace", ba.FontMonospace(), "font-monospace"},
		{"FlexRow", ba.FlexRow(), "flex-row"},
		{"FlexColumn", ba.FlexColumn(), "flex-column"},
		{"FlexRowReverse", ba.FlexRowReverse(), "flex-row-reverse"},
		{"FlexColumnReverse", ba.FlexColumnReverse(), "flex-column-reverse"},
		{"FlexWrap", ba.FlexWrap(), "flex-wrap"},
		{"FlexNowrap", ba.FlexNowrap(), "flex-nowrap"},
		{"FlexWrapReverse", ba.FlexWrapReverse(), "flex-wrap-reverse"},
		{"JustifyContentStart", ba.JustifyContentStart(), "justify-content-start"},
		{"JustifyContentEnd", ba.JustifyContentEnd(), "justify-content-end"},
		{"JustifyContentCenter", ba.JustifyContentCenter(), "justify-content-center"},
		{"JustifyContentBetween", ba.JustifyContentBetween(), "justify-content-between"},
		{"JustifyContentAround", ba.JustifyContentAround(), "justify-content-around"},
		{"JustifyContentEvenly", ba.JustifyContentEvenly(), "justify-content-evenly"},
		{"AlignItemsStart", ba.AlignItemsStart(), "align-items-start"},
		{"AlignItemsEnd", ba.AlignItemsEnd(), "align-items-end"},
		{"AlignItemsCenter", ba.AlignItemsCenter(), "align-items-center"},
		{"AlignItemsBaseline", ba.AlignItemsBaseline(), "align-items-baseline"},
		{"AlignItemsStretch", ba.AlignItemsStretch(), "align-items-stretch"},
		{"AlignSelfStart", ba.AlignSelfStart(), "align-self-start"},
		{"AlignSelfEnd", ba.AlignSelfEnd(), "align-self-end"},
		{"AlignSelfCenter", ba.AlignSelfCenter(), "align-self-center"},
		{"AlignSelfBaseline", ba.AlignSelfBaseline(), "align-self-baseline"},
		{"AlignSelfStretch", ba.AlignSelfStretch(), "align-self-stretch"},
		{"FlexFill", ba.FlexFill(), "flex-fill"},
		{"FlexGrow0", ba.FlexGrow0(), "flex-grow-0"},
		{"FlexGrow1", ba.FlexGrow1(), "flex-grow-1"},
		{"FlexShrink0", ba.FlexShrink0(), "flex-shrink-0"},
		{"FlexShrink1", ba.FlexShrink1(), "flex-shrink-1"},
		{"HStack", ba.HStack(), "hstack"},
		{"VStack", ba.VStack(), "vstack"},
		{"VR", ba.VR(), "vr"},
		{"FormControlSM", ba.FormControlSM(), "form-control-sm"},
		{"FormControlLG", ba.FormControlLG(), "form-control-lg"},
		{"FormSelectSM", ba.FormSelectSM(), "form-select-sm"},
		{"FormSelectLG", ba.FormSelectLG(), "form-select-lg"},
		{"IsInvalid", ba.IsInvalid(), "is-invalid"},
		{"IsValid", ba.IsValid(), "is-valid"},
		{"InvalidFeedback", ba.InvalidFeedback(), "invalid-feedback"},
		{"ValidFeedback", ba.ValidFeedback(), "valid-feedback"},
		{"WasValidated", ba.WasValidated(), "was-validated"},
		{"Border", ba.Border(), "border"},
		{"BorderTop", ba.BorderTop(), "border-top"},
		{"BorderEnd", ba.BorderEnd(), "border-end"},
		{"BorderBottom", ba.BorderBottom(), "border-bottom"},
		{"BorderStart", ba.BorderStart(), "border-start"},
		{"Border0", ba.Border0(), "border-0"},
		{"BorderPrimary", ba.BorderPrimary(), "border-primary"},
		{"BorderSecondary", ba.BorderSecondary(), "border-secondary"},
		{"BorderSuccess", ba.BorderSuccess(), "border-success"},
		{"BorderDanger", ba.BorderDanger(), "border-danger"},
		{"BorderWarning", ba.BorderWarning(), "border-warning"},
		{"BorderInfo", ba.BorderInfo(), "border-info"},
		{"BorderLight", ba.BorderLight(), "border-light"},
		{"BorderDark", ba.BorderDark(), "border-dark"},
		{"BorderBlack", ba.BorderBlack(), "border-black"},
		{"BorderWhite", ba.BorderWhite(), "border-white"},
		{"BorderPrimarySubtle", ba.BorderPrimarySubtle(), "border-primary-subtle"},
		{"BorderSecondarySubtle", ba.BorderSecondarySubtle(), "border-secondary-subtle"},
		{"BorderSuccessSubtle", ba.BorderSuccessSubtle(), "border-success-subtle"},
		{"BorderDangerSubtle", ba.BorderDangerSubtle(), "border-danger-subtle"},
		{"BorderWarningSubtle", ba.BorderWarningSubtle(), "border-warning-subtle"},
		{"BorderInfoSubtle", ba.BorderInfoSubtle(), "border-info-subtle"},
		{"BorderLightSubtle", ba.BorderLightSubtle(), "border-light-subtle"},
		{"BorderDarkSubtle", ba.BorderDarkSubtle(), "border-dark-subtle"},
		{"Rounded", ba.Rounded(), "rounded"},
		{"RoundedTop", ba.RoundedTop(), "rounded-top"},
		{"RoundedEnd", ba.RoundedEnd(), "rounded-end"},
		{"RoundedBottom", ba.RoundedBottom(), "rounded-bottom"},
		{"RoundedStart", ba.RoundedStart(), "rounded-start"},
		{"RoundedCircle", ba.RoundedCircle(), "rounded-circle"},
		{"RoundedPill", ba.RoundedPill(), "rounded-pill"},
		{"Rounded0", ba.Rounded0(), "rounded-0"},
		{"Shadow", ba.Shadow(), "shadow"},
		{"ShadowSM", ba.ShadowSM(), "shadow-sm"},
		{"ShadowLG", ba.ShadowLG(), "shadow-lg"},
		{"ShadowNone", ba.ShadowNone(), "shadow-none"},
		{"UserSelectAll", ba.UserSelectAll(), "user-select-all"},
		{"UserSelectAuto", ba.UserSelectAuto(), "user-select-auto"},
		{"UserSelectNone", ba.UserSelectNone(), "user-select-none"},
		{"PENone", ba.PENone(), "pe-none"},
		{"PEAuto", ba.PEAuto(), "pe-auto"},
		{"Container", ba.Container(), "container"},
		{"ContainerFluid", ba.ContainerFluid(), "container-fluid"},
		{"ContainerSM", ba.ContainerSM(), "container-sm"},
		{"ContainerMD", ba.ContainerMD(), "container-md"},
		{"ContainerLG", ba.ContainerLG(), "container-lg"},
		{"ContainerXL", ba.ContainerXL(), "container-xl"},
		{"ContainerXXL", ba.ContainerXXL(), "container-xxl"},
		{"Row", ba.Row(), "row"},
		{"Col", ba.Col(), "col"},
		{"Col1", ba.Col1(), "col-1"},
		{"Col2", ba.Col2(), "col-2"},
		{"Col3", ba.Col3(), "col-3"},
		{"Col4", ba.Col4(), "col-4"},
		{"Col5", ba.Col5(), "col-5"},
		{"Col6", ba.Col6(), "col-6"},
		{"Col7", ba.Col7(), "col-7"},
		{"Col8", ba.Col8(), "col-8"},
		{"Col9", ba.Col9(), "col-9"},
		{"Col10", ba.Col10(), "col-10"},
		{"Col11", ba.Col11(), "col-11"},
		{"Col12", ba.Col12(), "col-12"},
		{"ColAuto", ba.ColAuto(), "col-auto"},
	}
	for _, h := range intHelpers() {
		for n := h.lo; n <= h.hi; n++ {
			cases = append(cases, classCase{fmt.Sprintf("%s(%d)", h.name, n), h.fn(n), fmt.Sprintf("%s%d", h.prefix, n)})
		}
	}
	for _, h := range bpHelpers() {
		for _, bp := range breakpoints {
			cases = append(cases, classCase{fmt.Sprintf("%s(%s)", h.name, bp), h.fn(bp), fmt.Sprintf(h.pattern, bp)})
		}
	}
	return cases
}

func render(n elem.Node) (string, error) {
	var out strings.Builder
	err := core.Render(context.Background(), n, &out)
	return out.String(), err
}

// on renders the attributes on an empty <i> element.
func on(attrs ...attr.Node) (string, error) {
	return render(elem.Name("i").New(attrs...)())
}

func TestClasses(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, c := range classCases() {
		got, err := on(c.node)
		t.Mustf(be.NoError(err), "%s", c.name)
		t.Mustf(be.Equal(`<i class="`+c.want+`"></i>`, got), "%s", c.name)
	}
}

// TestClassesExistInBootstrap checks every produced class against the pinned
// Bootstrap stylesheet, so that no helper produces a class that does not
// exist.
func TestClassesExistInBootstrap(gt *testing.T) {
	t := devtest.ParallelT(gt)
	css, err := os.ReadFile(bootstrapCSS)
	t.Must(be.NoError(err))
	defined := make(map[string]bool)
	for _, m := range regexp.MustCompile(`\.([a-zA-Z][a-zA-Z0-9_-]*)`).FindAllSubmatch(css, -1) {
		defined[string(m[1])] = true
	}
	t.Must(be.True(defined["container"]))
	for _, c := range classCases() {
		for _, class := range strings.Fields(c.want) {
			t.Mustf(be.True(defined[class]), "%s: class %q is not in %s", c.name, class, bootstrapCSS)
		}
	}
}

// TestEveryHelperCovered fails when an exported helper has no case in the
// tables, so that new helpers get checked against Bootstrap too.
func TestEveryHelperCovered(gt *testing.T) {
	t := devtest.ParallelT(gt)
	covered := map[string]bool{"Tooltip": true, "Popover": true} // TestTooltipPopover
	for _, c := range classCases() {
		name, _, _ := strings.Cut(c.name, "(")
		covered[name] = true
	}
	entries, err := os.ReadDir(".")
	t.Must(be.NoError(err))
	fset := token.NewFileSet()
	n := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, e.Name(), nil, parser.SkipObjectResolution)
		t.Must(be.NoError(err))
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !fn.Name.IsExported() {
				continue
			}
			n++
			t.Mustf(be.True(covered[fn.Name.Name]), "helper %s has no test case", fn.Name.Name)
		}
	}
	t.Must(be.Greater(n, 300))
}

// mustFail checks that the attribute fails the render with ErrInvalidValue,
// and that the error does not contain leak.
func mustFail(t devtest.T, name string, node attr.Node, leak string) {
	t.Helper()
	got, err := on(node)
	t.Mustf(be.ErrorIs(err, ba.ErrInvalidValue), "%s: got %q", name, got)
	t.Mustf(be.ErrorContains(err, "ba: "), "%s", name)
	if leak != "" {
		t.Mustf(be.False(strings.Contains(err.Error(), leak)), "%s: error repeats the value: %v", name, err)
	}
}

func TestInvalidInts(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, h := range intHelpers() {
		for _, n := range []int{h.lo - 1, h.hi + 1, -1, 99, math.MinInt, math.MaxInt} {
			if n >= h.lo && n <= h.hi {
				continue
			}
			mustFail(t, fmt.Sprintf("%s(%d)", h.name, n), h.fn(n), "")
		}
	}
	_, err := on(ba.MT(-1))
	t.Must(be.Equal("render: at i[0]: attribute: ba: mt--1: invalid utility value (want 0 to 5)", err.Error()))
	_, err = on(ba.Offset(0))
	t.Must(be.ErrorContains(err, "offset-0: invalid utility value (want 1 to 11)"))
}

func TestInvalidBreakpoints(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, h := range bpHelpers() {
		for _, bp := range []ba.Breakpoint{0, ba.XXL + 1, 255} {
			mustFail(t, fmt.Sprintf("%s(%d)", h.name, bp), h.fn(bp), "")
		}
	}
	// A bad breakpoint is reported before a bad number.
	_, err := on(ba.ColAt(0, 13))
	t.Must(be.ErrorContains(err, "ba: col-{bp}-13: invalid utility value: Breakpoint(0) (want SM to XXL)"))
	_, err = on(ba.DNoneAt(9))
	t.Must(be.ErrorContains(err, "ba: d-{bp}-none: invalid utility value: Breakpoint(9)"))
	_, err = on(ba.ColAt(ba.MD, 13))
	t.Must(be.ErrorContains(err, "ba: col-md-13: invalid utility value (want 1 to 12)"))
}

func TestInvalidNames(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, v := range []string{"", "Bold", "bold ", "900", "bold\" onclick=\"x()", "<b>", "fw-bold", "secret-token-123"} {
		mustFail(t, fmt.Sprintf("FW(%q)", v), ba.FW(v), nonEmptyLeak(v))
	}
	for _, v := range []string{"", "2", "normal", "LG", "lg\" x=\"y", "secret-token-123"} {
		mustFail(t, fmt.Sprintf("LH(%q)", v), ba.LH(v), nonEmptyLeak(v))
	}
	_, err := on(ba.FW("x"))
	t.Must(be.ErrorContains(err, `ba: fw-*: invalid utility value (want one of ["bold" "bolder" "semibold" "medium" "normal" "light" "lighter"])`))
}

// nonEmptyLeak returns the value to look for in an error message, skipping
// values that legitimately appear in it (the allowed list or the prefix).
func nonEmptyLeak(v string) string {
	if len(v) < 4 || strings.Contains(`["bold" "bolder" "semibold" "medium" "normal" "light" "lighter"] fw-* lh-*`, v) {
		return ""
	}
	return v
}

func TestBreakpointString(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for bp, want := range map[ba.Breakpoint]string{
		ba.SM: "sm", ba.MD: "md", ba.LG: "lg", ba.XL: "xl", ba.XXL: "xxl",
		0: "Breakpoint(0)", 6: "Breakpoint(6)", 255: "Breakpoint(255)",
	} {
		t.Must(be.Equal(want, bp.String()))
	}
}

func TestMergedClassAttribute(gt *testing.T) {
	t := devtest.ParallelT(gt)
	got, err := on(ba.DFlex(), attr.ID("toolbar"), ba.Gap(2), attr.Class(`x" onclick="y`), ba.MSAuto(), ba.FormControl(ba.IsInvalid()))
	t.Must(be.NoError(err))
	t.Must(be.Equal(`<i class="d-flex gap-2 x&#34; onclick=&#34;y ms-auto form-control is-invalid" id="toolbar"></i>`, got))
	t.Must(be.Equal(1, strings.Count(got, "class=")))
}

func TestFormHelpersCarryOptions(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for name, c := range map[string]struct {
		node attr.Node
		want string
	}{
		"FormControl":    {ba.FormControl(ba.FormControlSM(), attr.ID("title")), `<i class="form-control form-control-sm" id="title"></i>`},
		"FormSelect":     {ba.FormSelect(ba.FormSelectLG(), attr.Name("aria-invalid").Value("true")), `<i class="form-select form-select-lg" aria-invalid="true"></i>`},
		"FormCheckInput": {ba.FormCheckInput(ba.IsValid()), `<i class="form-check-input is-valid"></i>`},
	} {
		got, err := on(c.node)
		t.Must(be.NoError(err))
		t.Mustf(be.Equal(c.want, got), "%s", name)
	}
	// An invalid option fails the whole attribute.
	_, err := on(ba.FormControl(ba.MT(7)))
	t.Must(be.ErrorIs(err, ba.ErrInvalidValue))
}

func TestTooltipPopover(gt *testing.T) {
	t := devtest.ParallelT(gt)
	got, err := on(ba.Tooltip(attr.Title("Copy")))
	t.Must(be.NoError(err))
	t.Must(be.Equal(`<i data-bs-toggle="tooltip" title="Copy"></i>`, got))
	got, err = on(ba.Popover(attr.Title("Details"), attr.Data("bs-content", `a "quoted" <text>`)))
	t.Must(be.NoError(err))
	t.Must(be.Equal(`<i data-bs-toggle="popover" title="Details" data-bs-content="a &#34;quoted&#34; &lt;text&gt;"></i>`, got))
	got, err = on(ba.Tooltip())
	t.Must(be.NoError(err))
	t.Must(be.Equal(`<i data-bs-toggle="tooltip"></i>`, got))
}

func TestNodesAreReusable(gt *testing.T) {
	t := devtest.ParallelT(gt)
	good := elem.Name("i").New(ba.MT(3), ba.DNoneAt(ba.MD), ba.FormControl(ba.IsInvalid()))()
	bad := elem.Name("i").New(ba.MT(30))()
	var wg sync.WaitGroup
	results := make([]error, 16)
	for i := range results {
		wg.Go(func() {
			got, err := render(good)
			if err == nil && got != `<i class="mt-3 d-md-none form-control is-invalid"></i>` {
				err = errors.New("unexpected output " + got)
			}
			if err == nil {
				if _, badErr := render(bad); !errors.Is(badErr, ba.ErrInvalidValue) {
					err = fmt.Errorf("invalid node rendered: %v", badErr)
				}
			}
			results[i] = err
		})
	}
	wg.Wait()
	for _, err := range results {
		t.Must(be.NoError(err))
	}
}
