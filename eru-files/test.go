package eru_files

import (
	"fmt"
	store "github.com/eru-tech/eru/eru-store"
)

func main() {
	// Get a greeting message and print it.
	message := store.Hello("Gladys")
	fmt.Println(message)
}
