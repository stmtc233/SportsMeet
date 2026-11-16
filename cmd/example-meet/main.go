// Export the same example that is bundled in the app:
// go run ./cmd/example-meet > example.meet.json
package main

import (
	"fmt"
	"os"

	"github.com/stmtc233/sportsmeet/internal/examplemeet"
	"github.com/stmtc233/sportsmeet/internal/interchange"
)

func main() {
	data, err := interchange.Encode(examplemeet.Build())
	if err == nil {
		_, err = interchange.Decode(data)
	}
	if err == nil {
		_, err = os.Stdout.Write(data)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
