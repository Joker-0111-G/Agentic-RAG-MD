package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"Agentic-RAG-MD/global"
	"Agentic-RAG-MD/models"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

type FrontMatter struct {
	Title string   `yaml:"title"`
	Tags  []string `yaml:"tags"`
}

func parseMarkdown(filePath string) (title string, tags string, wordCount int, err error) {
	contentBytes, err := os.ReadFile(filePath)
	if err != nil { return "", "", 0, err }
	contentStr := string(contentBytes)

	wordCount = utf8.RuneCountInString(contentStr)

	if strings.HasPrefix(contentStr, "---\n") || strings.HasPrefix(contentStr, "---\r\n") {
		parts := strings.SplitN(contentStr, "---", 3)
		if len(parts) >= 3 {
			var fm FrontMatter
			if err := yaml.Unmarshal([]byte(parts[1]), &fm); err == nil {
				title = strings.TrimSpace(fm.Title)
				tags = strings.Join(fm.Tags, ",")
			}
		}
	}
	return title, tags, wordCount, nil
}

func UploadDocumentHandler(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "获取上传文件失败: " + err.Error()})
		return
	}

	uploadDir := "./uploads/markdowns"
	os.MkdirAll(uploadDir, os.ModePerm)
	
	newFileName := fmt.Sprintf("%d_%s", time.Now().Unix(), file.Filename)
	savePath := filepath.Join(uploadDir, newFileName)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败: " + err.Error()})
		return
	}

	title, tags, wordCount, _ := parseMarkdown(savePath)
	if title == "" {
		title = strings.TrimSuffix(file.Filename, filepath.Ext(file.Filename))
	}

	doc := models.Document{ FileName: file.Filename, Title: title, FilePath: savePath, Tags: tags, WordCount: wordCount }
	if err := global.DB.Create(&doc).Error; err != nil {
		_ = os.Remove(savePath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库记录创建失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "文档上传并解析成功", "data": doc})
}

func DeleteDocumentHandler(c *gin.Context) {
	docID := c.Param("id")
	var doc models.Document
	
	if err := global.DB.First(&doc, docID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到该文档记录"})
		return
	}
	if err := os.Remove(doc.FilePath); err != nil && !os.IsNotExist(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除本地文件失败: " + err.Error()})
		return
	}
	if err := global.DB.Delete(&doc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除数据库记录失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "文档删除成功", "deleted_id": doc.ID})
}