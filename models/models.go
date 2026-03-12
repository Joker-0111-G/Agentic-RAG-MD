package models

import (
	"time"
	"gorm.io/gorm"
)

// Document 对应 documents 表，存储 Markdown 文件元数据
type Document struct {
	ID        uint           `gorm:"primarykey"`
	FileName  string         `gorm:"type:varchar(255);not null;comment:'原始文件名'"`
	Title     string         `gorm:"type:varchar(255);index;comment:'文档标题(从YAML提取或同文件名)'"`
	FilePath  string         `gorm:"type:varchar(512);not null;comment:'本地存储路径或OSS地址'"`
	Tags      string         `gorm:"type:varchar(255);index;comment:'文档标签，逗号分隔或JSON格式，方便检索'"`
	WordCount int            `gorm:"type:int;comment:'字数统计'"`
	Summary   string         `gorm:"type:text;comment:'文档生成时的AI简短摘要(可选)'"`
	CreatedAt time.Time      `gorm:"index;comment:'上传/创建时间'"`
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// ChatHistory 对应 chat_history 表，持久化存储 Agent 对话记录
type ChatHistory struct {
	ID        uint           `gorm:"primarykey"`
	SessionID string         `gorm:"type:varchar(64);not null;index;comment:'会话ID，用于串联多轮对话'"`
	Role      string         `gorm:"type:varchar(20);not null;comment:'角色: user, assistant, system, tool'"`
	Content   string         `gorm:"type:text;not null;comment:'对话内容或工具返回结果'"`
	ToolName  string         `gorm:"type:varchar(64);comment:'如果是tool调用，记录调用的工具名称(可选)'"`
	CreatedAt time.Time      `gorm:"index;comment:'消息发生时间'"`
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// ChatSession 对应 chat_sessions 表，管理左侧的会话列表
type ChatSession struct {
	ID        string         `gorm:"type:varchar(64);primarykey;comment:'会话唯一标识(UUID/NanoID)'"`
	Title     string         `gorm:"type:varchar(100);not null;default:'新对话';comment:'会话标题(可由LLM生成)'"`
	UserID    uint           `gorm:"index;comment:'预留用户ID字段，方便后续扩展多用户'"`
	CreatedAt time.Time      `gorm:"index;comment:'会话创建时间'"`
	UpdatedAt time.Time      `gorm:"index;comment:'最后活跃时间，用于列表排序'"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}