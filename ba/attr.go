package ba

import "github.com/protolambda/chord/core/attr"

func rawClass(value string) attr.Node {
	return attr.Name("class").Raw(value)
}
