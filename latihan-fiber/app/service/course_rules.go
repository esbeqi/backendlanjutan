package service

import "errors"

const MaxCreditsPerSemester = 24

var errCreditLimit = errors.New("batas SKS terlampaui")

func WithinCreditLimit(taken, adding int) bool {
	return taken+adding <= MaxCreditsPerSemester
}

func CreditsAfterSwap(taken, leaving, joining int) int {
	return taken - leaving + joining
}
