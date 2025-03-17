package main

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"net/http"
	"strings"
	"time"
)

// 智能嵌套结构体定义（自动扩展数据层）
type MegaData struct {
	ID       string      `json:"id"`
	Content  ContentData `json:"content"`
	Metadata MetaLayer   `json:"metadata"` // 嵌套元数据层
}

type ContentData struct {
	TextBlock  string   `json:"text"`
	VectorData []string `json:"vectors"` // 向量数据自动填充
}

type MetaLayer struct {
	Tags      []string          `json:"tags"`
	Embedding map[string]string `json:"embedding"` // 动态键值对
}

func main() {
	e := echo.New()
	configureServer(e) // 服务器性能调优

	e.GET("/big-json", generateBigJSON)
	e.Logger.Fatal(e.Start(":8080"))
}

// 生成超过7MB的结构体（内存高效方式）
func generateBigJSON(c echo.Context) error {
	// 智能生成层级数据
	data := MegaData{
		ID: "7MB_DATA_MODEL",
		Content: ContentData{
			TextBlock:  generateText(5 * 1024 * 1024), // 生成5MB基础文本
			VectorData: generateVectors(1024),         // 生成1024个向量
		},
		Metadata: buildMetaLayer(5000), // 构建5000个元数据项
	}
	return c.JSON(http.StatusOK, data)
}

// 智能文本生成器（流式构建）
func generateText(targetSize int) string {
	builder := strings.Builder{}
	base := "AI生成文本-"

	// 计算需要重复次数
	unitSize := len(base)
	repeat := targetSize / unitSize

	builder.Grow(targetSize) // 预分配内存
	for i := 0; i < repeat; i++ {
		builder.WriteString(base)
	}
	return builder.String()
}

// 向量数据生成器（避免内存爆炸）
func generateVectors(count int) []string {
	vectors := make([]string, count)
	base := []string{"vec1", "vec2", "vec3"} // 基础向量模板

	for i := range vectors {
		// 动态生成向量标识
		vectors[i] = strings.Join([]string{
			base[i%3],
			string(rune('A' + i%26)), // 自动生成后缀
		}, "-")
	}
	return vectors
}

// 元数据层构建器（智能键值生成）
func buildMetaLayer(items int) MetaLayer {
	meta := MetaLayer{
		Tags:      make([]string, items/2),
		Embedding: make(map[string]string, items/2),
	}

	// 自动生成标签系统
	for i := range meta.Tags {
		meta.Tags[i] = strings.Join([]string{
			"tag",
			string(rune('A' + i%26)), // 自动生成字母后缀
			string(rune('0' + i%10)), // 数字标识
		}, "-")
	}

	// 生成动态键值对
	for i := 0; i < items/2; i++ {
		key := strings.Join([]string{
			"key",
			string(rune('A' + i%26)),
			string(rune('a' + i%26)),
		}, "")
		meta.Embedding[key] = strings.Repeat("X", 100) // 每个值100字节
	}
	return meta
}

// 服务器性能优化配置
func configureServer(e *echo.Echo) {
	e.Server = &http.Server{
		MaxHeaderBytes: 1 << 20, // 1MB头限制
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   60 * time.Second,
		IdleTimeout:    300 * time.Second,
	}
	e.Use(middleware.Recover())
	e.HideBanner = true
}
