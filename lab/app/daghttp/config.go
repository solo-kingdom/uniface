package daghttp

const (
	// DefaultFixturesDir 是 daghttp 域 fixture 的默认相对路径（相对 cwd）。
	// 调用方如未显式提供 cfg.FixturesDir 可使用本常量（或由 LoadConfig 注入）。
	DefaultFixturesDir = "app/daghttp/fixtures/graphs"

	// defaultGraphID 是 echo DAG 的默认图 ID。
	defaultGraphID = "echo"
)

// Config 是 daghttp 域的运行时配置 schema。
//
// 注意：FixturesDir 是 daghttp 的核心字段——在 LoadConfig 中若为空会回退到
// DefaultFixturesDir；调用方可直接通过本类型显式指定。
type Config struct {
	Store       string `yaml:"store"`
	FixturesDir string `yaml:"fixtures_dir"`
}
