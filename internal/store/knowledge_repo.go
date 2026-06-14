package store

import (
	"go-agent-chat-server/internal/model"

	"gorm.io/gorm"
)

type KnowledgeRepo struct {
	db *gorm.DB
}

// KnowledgeRepo持有一个GORM数据库连接对象
func NewKnowledgeRepo(db *gorm.DB) *KnowledgeRepo {
	return &KnowledgeRepo{db: db}
}

func (r *KnowledgeRepo) Create(doc *model.KnowledgeDoc) error { //新增知识文档
	return r.db.Create(doc).Error
}

func (r *KnowledgeRepo) ListByUserID(userID string, page int, pageSize int) ([]model.KnowledgeDoc, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	docs := make([]model.KnowledgeDoc, 0)     //用来接收数据库查询结果
	err := r.db.Where("user_id = ?", userID). //查询用户的文档，分页Offset
							Order("created_at desc").
							Offset((page - 1) * pageSize).
							Limit(pageSize).
							Find(&docs).Error //Find执行查询
	return docs, err
}

func (r *KnowledgeRepo) CountByUserID(userID string) (int64, error) {
	var total int64
	err := r.db.Model(&model.KnowledgeDoc{}).Where("user_id = ?", userID).Count(&total).Error
	return total, err
}

func (r *KnowledgeRepo) GetByIDAndUserID(id string, userID string) (model.KnowledgeDoc, error) {
	var doc model.KnowledgeDoc
	err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&doc).Error
	if err != nil {
		return model.KnowledgeDoc{}, err
	}
	return doc, nil
} //根据文档ID和用户ID查询单篇文档

func (r *KnowledgeRepo) DeleteByIDAndUserID(id string, userID string) error {
	result := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.KnowledgeDoc{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
} //删除当前用户的一篇文档，0说明没有找到符合条件的文档

func (r *KnowledgeRepo) Search(userID string, keyword string, limit int) ([]model.KnowledgeDoc, error) {
	if limit <= 0 {
		limit = 5
	} //关键词搜索知识库文档，默认5条

	docs := make([]model.KnowledgeDoc, 0)
	query := r.db.Where("user_id = ?", userID)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("title LIKE ? OR content LIKE ?", like, like)
	}

	err := query.Order("created_at desc").Limit(limit).Find(&docs).Error
	return docs, err
}
