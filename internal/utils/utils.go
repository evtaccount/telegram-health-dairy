package utils

import "log"

func LogFor(e error) {
	if e != nil {
		log.Fatal(e)
	}
}
