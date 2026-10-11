package mysql

import (
	"campusclaw/internal/material"
	"context"
	"database/sql"
	"errors"
)

func (r Materials) List(ctx context.Context, classID int64) ([]material.Item, error) {
	if classID <= 0 {
		return nil, material.ErrForbidden
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT m.id,m.original_name,m.size,m.created_at,u.username FROM materials m JOIN users u ON u.id=m.uploader_id WHERE m.class_id=? ORDER BY m.created_at DESC,m.id DESC`, classID)
	if err != nil {
		return nil, material.ErrUnavailable
	}
	defer rows.Close()
	items := []material.Item{}
	for rows.Next() {
		var v material.Item
		if rows.Scan(&v.ID, &v.OriginalName, &v.Size, &v.CreatedAt, &v.Uploader) != nil {
			return nil, material.ErrUnavailable
		}
		items = append(items, v)
	}
	if rows.Err() != nil {
		return nil, material.ErrUnavailable
	}
	return items, nil
}
func (r Materials) Find(ctx context.Context, classID int64, id string) (material.Detail, error) {
	if classID <= 0 {
		return material.Detail{}, material.ErrForbidden
	}
	var v material.Detail
	err := r.DB.QueryRowContext(ctx, `SELECT m.id,m.original_name,m.size,m.created_at,u.username,k.plain_text,m.storage_key FROM materials m JOIN users u ON u.id=m.uploader_id JOIN knowledge_entries k ON k.material_id=m.id AND k.class_id=m.class_id WHERE m.id=? AND m.class_id=?`, id, classID).Scan(&v.ID, &v.OriginalName, &v.Size, &v.CreatedAt, &v.Uploader, &v.PlainText, &v.StorageKey)
	if errors.Is(err, sql.ErrNoRows) {
		return material.Detail{}, material.ErrNotFound
	}
	if err != nil {
		return material.Detail{}, material.ErrUnavailable
	}
	return v, nil
}
