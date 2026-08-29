package main

import (
	"fmt"
	treeeditdistance "interpreter/treeEditDistance"
	// "interpreter/repl"
	// "os"
	"os/user"
)

func main() {
	user, err := user.Current()

	if err != nil {
		panic(err)
	}

	fmt.Printf("Hello %s! This is the mokey programming language!\n", user.Username)
	fmt.Printf("Feel free to tye in commands\n")
	//repl.Start(os.Stdin, os.Stdout)

	treeeditdistance.CreateSimilarTrees()

}
