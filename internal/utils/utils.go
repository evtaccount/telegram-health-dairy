package utils

import "log"

func Must(e error) {
	if e != nil {
		log.Fatal(e)
	}
}
