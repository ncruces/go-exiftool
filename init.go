/*
Package exiftool provides a thin wrapper around ExifTool.
*/
package exiftool

// Configure ExifTool.
var (
	Exec   string = "exiftool" // Application to execute.
	Arg1   string              // Optional first argument.
	Config string              // ExifTool config file to use.
)
