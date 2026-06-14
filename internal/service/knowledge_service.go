package service

import (
	"errors"
	"strings"

	"go-agent-chat-server/internal/model"
	"go-agent-chat-server/internal/pkg/idgen"
	"go-agent-chat-server/internal/store"
)

type KnowledgeService struct {
	knowledgeRepo *store.KnowledgeRepo
}

func NewKnowledgeService(knowledgeRepo *store.KnowledgeRepo) *KnowledgeService {
	return &KnowledgeService{knowledgeRepo: knowledgeRepo}
}

func (s *KnowledgeService) CreateDoc(userID string, title string, content string) (model.KnowledgeDoc, error) {
	if userID == "" { //检查userid
		return model.KnowledgeDoc{}, errors.New("user_id is required")
	}

	title = strings.TrimSpace(title) //清理标题和内容前后空格，判断是不是空内容，不去读掉中间的
	content = strings.TrimSpace(content)
	if title == "" {
		title = "Untitled"
	}
	if content == "" {
		return model.KnowledgeDoc{}, errors.New("content is required")
	}

	doc := model.KnowledgeDoc{
		ID:      idgen.NewID(),
		UserID:  userID,
		Title:   title,
		Content: content,
	}
	if err := s.knowledgeRepo.Create(&doc); err != nil {
		return model.KnowledgeDoc{}, err
	}
	return doc, nil
}

func (s *KnowledgeService) ListDocs(userID string, page int, pageSize int) ([]model.KnowledgeDoc, int64, error) {
	if userID == "" {
		return []model.KnowledgeDoc{}, 0, errors.New("user_id is required")
	}

	total, err := s.knowledgeRepo.CountByUserID(userID) //查询知识文档总数量
	if err != nil {
		return []model.KnowledgeDoc{}, 0, err
	}

	docs, err := s.knowledgeRepo.ListByUserID(userID, page, pageSize) //查询当前页的数据
	if err != nil {
		return []model.KnowledgeDoc{}, 0, err
	}

	return docs, total, nil //返回docs和total
}

func (s *KnowledgeService) GetDoc(userID string, id string) (model.KnowledgeDoc, error) {
	if userID == "" { //检查用户id
		return model.KnowledgeDoc{}, errors.New("user_id is required")
	}
	if id == "" {
		return model.KnowledgeDoc{}, errors.New("doc_id is required")
	}
	return s.knowledgeRepo.GetByIDAndUserID(id, userID)
}

func (s *KnowledgeService) DeleteDoc(userID string, id string) error {
	if userID == "" {
		return errors.New("user_id is required")
	}
	if id == "" {
		return errors.New("doc_id is required")
	}
	return s.knowledgeRepo.DeleteByIDAndUserID(id, userID)
}

func (s *KnowledgeService) SearchDocs(userID string, keyword string, limit int) ([]model.KnowledgeDoc, error) {
	if userID == "" {
		return []model.KnowledgeDoc{}, errors.New("user_id is required")
	}
	return s.knowledgeRepo.Search(userID, strings.TrimSpace(keyword), limit)
}
