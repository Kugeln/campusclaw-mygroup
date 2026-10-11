CREATE TABLE IF NOT EXISTS materials (
 id CHAR(36) NOT NULL PRIMARY KEY,
 class_id BIGINT NOT NULL, uploader_id BIGINT NOT NULL,
 original_name VARCHAR(255) NOT NULL, size BIGINT NOT NULL,
 content_type VARCHAR(100) NOT NULL, storage_key CHAR(36) NOT NULL UNIQUE,
 created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 UNIQUE KEY material_scope(id,class_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS knowledge_entries (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
 material_id CHAR(36) NOT NULL UNIQUE, class_id BIGINT NOT NULL,
 body_text LONGTEXT NOT NULL, created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 index_status ENUM('pending','indexing','ready','failed') NOT NULL DEFAULT 'pending',
 chunk_strategy JSON NULL, cleanup_required BOOLEAN NOT NULL DEFAULT FALSE,
 UNIQUE KEY entry_scope(id,material_id,class_id),
 KEY entry_status(class_id,index_status),
 FOREIGN KEY(material_id,class_id) REFERENCES materials(id,class_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS knowledge_chunks (
 id CHAR(36) NOT NULL PRIMARY KEY, knowledge_entry_id BIGINT NOT NULL,
 material_id CHAR(36) NOT NULL, class_id BIGINT NOT NULL,
 chunk_index INT NOT NULL, chunk_text TEXT NOT NULL,
 index_status ENUM('pending','ready','failed') NOT NULL DEFAULT 'pending',
 UNIQUE KEY entry_position(knowledge_entry_id,chunk_index),
 KEY chunk_status(class_id,index_status),
 FOREIGN KEY(knowledge_entry_id,material_id,class_id) REFERENCES knowledge_entries(id,material_id,class_id),
 FULLTEXT KEY chunk_fulltext(chunk_text) WITH PARSER ngram
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
