module github.com/Defacto2/archive

go 1.26.8

require (
	github.com/Defacto2/helper v1.7.3
	github.com/Defacto2/magicnumber v1.4.5
	github.com/nalgeon/be v0.3.0
)

//	replace github.com/Defacto2/helper => ../helper
//	replace github.com/Defacto2/magicnumber => ../magicnumber

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	go.uber.org/nilaway v0.0.0-20251119034912-44f92224c998 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	golang.org/x/tools v0.49.0 // indirect
)

tool go.uber.org/nilaway/cmd/nilaway
