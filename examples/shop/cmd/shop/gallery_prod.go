//go:build !gxdev

package main

// setupGallery does nothing in a production build (SI-08).
func setupGallery() {}
