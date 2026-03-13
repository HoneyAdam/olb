package utils

import "errors"

// Sentinel errors for utilities.
var (
	ErrInvalidCapacity    = errors.New("utils: invalid buffer capacity")
	ErrBufferFull         = errors.New("utils: buffer is full")
	ErrBufferEmpty        = errors.New("utils: buffer is empty")
	ErrIncompatibleFilter = errors.New("utils: incompatible bloom filter")
)
