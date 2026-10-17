// Package ba provides Bootstrap 5.3 attribute utilities.
//
// This package contains Bootstrap class attributes that can be applied to
// HTML elements. Use these with element constructors from the html packages
// or from the bs package.
//
// # Why Separate Packages?
//
// Bootstrap utilities are split into two packages for clarity:
//   - [bs]: Element components that create HTML elements (buttons, cards, rows, etc.)
//   - ba: Attribute utilities that add classes to existing elements
//
// This separation prevents confusion: bs.Row() creates a <div>, while ba.Row()
// returns a class attribute you can apply to any element.
//
// # Usage
//
// Apply attributes to HTML elements:
//
//	// With html elements
//	div.Div(ba.MT(3), ba.P(2), ba.BgLight(), text.Text("Styled"))
//
//	// With bs elements
//	bs.Container(ba.MT(4), ba.Shadow(), ...)
//	bs.Row(ba.MB(3), bs.Col6(...), bs.Col6(...))
//
//	// Combine multiple utilities
//	div.Div(ba.DFlex(), ba.JustifyContentBetween(), ba.AlignItemsCenter(), ba.Gap(3))
//
// # Grid Attributes
//
// Grid class attributes for use with custom elements:
//
//	ba.Container()      // "container"
//	ba.ContainerFluid() // "container-fluid"
//	ba.Row()            // "row"
//	ba.Col()            // "col"
//	ba.Col6()           // "col-6"
//	ba.ColMD(4)         // "col-md-4"
//	ba.ColLG(3)         // "col-lg-3"
//	ba.Offset(2)        // "offset-2"
//	ba.OffsetMD(1)      // "offset-md-1"
//
// # Spacing
//
// Margin and padding utilities (values 0-5, or "auto"):
//
//	ba.M(3)   // margin: 1rem
//	ba.MT(4)  // margin-top
//	ba.MB(2)  // margin-bottom
//	ba.MS(1)  // margin-start (left in LTR)
//	ba.ME(1)  // margin-end (right in LTR)
//	ba.MX(3)  // margin left+right
//	ba.MY(2)  // margin top+bottom
//
//	ba.P(3)   // padding: 1rem
//	ba.PT(4)  // padding-top
//	ba.PB(2)  // padding-bottom
//	ba.PS(1)  // padding-start
//	ba.PE(1)  // padding-end
//	ba.PX(3)  // padding left+right
//	ba.PY(2)  // padding top+bottom
//
// # Flexbox
//
// Flexbox utilities for layout:
//
//	ba.DFlex()                // display: flex
//	ba.DInlineFlex()          // display: inline-flex
//	ba.FlexRow()              // flex-direction: row
//	ba.FlexColumn()           // flex-direction: column
//	ba.FlexWrap()             // flex-wrap: wrap
//	ba.JustifyContentStart()  // justify-content: flex-start
//	ba.JustifyContentCenter() // justify-content: center
//	ba.JustifyContentBetween()// justify-content: space-between
//	ba.AlignItemsStart()      // align-items: flex-start
//	ba.AlignItemsCenter()     // align-items: center
//	ba.AlignSelfEnd()         // align-self: flex-end
//	ba.FlexGrow1()            // flex-grow: 1
//	ba.FlexShrink0()          // flex-shrink: 0
//	ba.Gap(3)                 // gap: 1rem
//
// # Display
//
// Display utilities:
//
//	ba.DNone()        // display: none
//	ba.DBlock()       // display: block
//	ba.DInline()      // display: inline
//	ba.DInlineBlock() // display: inline-block
//	ba.DFlex()        // display: flex
//	ba.DGrid()        // display: grid
//
// # Colors
//
// Background and text colors:
//
//	ba.BgPrimary()    // background-color: primary
//	ba.BgSecondary()  // background-color: secondary
//	ba.BgSuccess()    // background-color: success
//	ba.BgDanger()     // background-color: danger
//	ba.BgWarning()    // background-color: warning
//	ba.BgLight()      // background-color: light
//	ba.BgDark()       // background-color: dark
//
//	ba.TextPrimary()  // color: primary
//	ba.TextMuted()    // color: muted (text-body-secondary)
//	ba.TextWhite()    // color: white
//
// # Borders
//
// Border utilities:
//
//	ba.Border()        // border on all sides
//	ba.BorderTop()     // border-top only
//	ba.Border0()       // remove border
//	ba.BorderPrimary() // border-color: primary
//	ba.Rounded()       // border-radius
//	ba.RoundedCircle() // border-radius: 50%
//	ba.RoundedPill()   // pill shape
//
// # Sizing
//
// Width and height utilities:
//
//	ba.W25(), ba.W50(), ba.W75(), ba.W100(), ba.WAuto()
//	ba.H25(), ba.H50(), ba.H75(), ba.H100(), ba.HAuto()
//	ba.MW100()    // max-width: 100%
//	ba.MH100()    // max-height: 100%
//	ba.VWMin100() // min-width: 100vw
//	ba.VHMin100() // min-height: 100vh
//
// # Position
//
// Position utilities:
//
//	ba.PositionRelative()
//	ba.PositionAbsolute()
//	ba.PositionFixed()
//	ba.PositionSticky()
//	ba.Top0(), ba.Top50(), ba.Top100()
//	ba.Start0(), ba.Start50(), ba.End0()
//	ba.TranslateMiddle()
//
// # Shadows
//
//	ba.Shadow()     // default shadow
//	ba.ShadowSM()   // small shadow
//	ba.ShadowLG()   // large shadow
//	ba.ShadowNone() // no shadow
//
// # Text
//
// Text utilities:
//
//	ba.TextStart(), ba.TextCenter(), ba.TextEnd()
//	ba.TextWrap(), ba.TextNowrap(), ba.TextBreak()
//	ba.TextUppercase(), ba.TextLowercase(), ba.TextCapitalize()
//	ba.FWBold(), ba.FWNormal(), ba.FWLight()
//	ba.FSItalic(), ba.FSNormal()
//	ba.FontMonospace()
//
// # Forms
//
// Form input styling:
//
//	ba.FormControl()    // style input/textarea as form-control
//	ba.FormSelect()     // style select as form-select
//	ba.FormCheckInput() // style checkbox/radio input
//
// # Interactive
//
// Tooltip and popover triggers:
//
//	ba.Tooltip()  // data-bs-toggle="tooltip"
//	ba.Popover()  // data-bs-toggle="popover"
//
// # Other
//
//	ba.Visible(), ba.Invisible()
//	ba.OverflowAuto(), ba.OverflowHidden(), ba.OverflowScroll()
//	ba.UserSelectNone(), ba.UserSelectAll()
//	ba.PENone(), ba.PEAuto() // pointer-events
//
// Version: Bootstrap 5.3
// Reference: https://getbootstrap.com/docs/5.3/
package ba
