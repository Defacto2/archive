module github.com/Defacto2/archive

go 1.27.3

require (
	github.com/Defacto2/helper v1.7.5
	github.com/Defacto2/magicnumber v1.4.6
	github.com/nalgeon/be v0.3.0
)

//	replace github.com/Defacto2/helper => ../helper
//	replace github.com/Defacto2/magicnumber => ../magicnumber

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.20.0 // indirect
	go.uber.org/nilaway v0.0.0-20260918162853-acb8859b9031 // indirect
	golang.org/x/exp/typeparams v0.0.0-20260908205506-85c1c2202aba // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	golang.org/x/tools v0.51.0 // indirect
)

tool go.uber.org/nilaway/cmd/nilaway
