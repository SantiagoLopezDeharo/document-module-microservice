package main

import (
	"fmt"
	"strings"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/codec"
)

func main() {
	xmlStr := `<doc>
    <List>
        <a>1</a>
        <b>2</b>
        <c>9</c>
    </List>
    <List>
        <a>6</a>
        <b>7</b>
    </List>
</doc>`

	c, _ := codec.GetCodec("XML")
	res, err := c.Decode(strings.NewReader(xmlStr))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Result: %+v\n", res)
}
