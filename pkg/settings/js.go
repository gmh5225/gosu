package settings

type js struct {
	Engine     string `json:"engine"`     // node, deno, etc.
	Transpiler string `json:"transpiler"` // tsx, ts-node, ts-node/esm etc.
}

var Javascript = Settings(js{
	Engine:     "node",
	Transpiler: "tsx",
})
