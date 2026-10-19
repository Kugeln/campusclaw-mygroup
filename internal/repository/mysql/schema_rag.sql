ALTER TABLE knowledge_entries ADD COLUMN index_status ENUM('pending','indexing','ready','failed') NOT NULL DEFAULT 'pending';
ALTER TABLE knowledge_entries ADD COLUMN chunk_strategy JSON NULL;
ALTER TABLE knowledge_entries ADD COLUMN cleanup_required BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE knowledge_entries ADD UNIQUE rag_entry_scope (id,material_id,class_id);
ALTER TABLE knowledge_entries ADD INDEX rag_entry_status (class_id,index_status);
CREATE TABLE IF NOT EXISTS knowledge_chunks (
 id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
 knowledge_entry_id BIGINT NOT NULL,
 material_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 class_id BIGINT NOT NULL,
 chunk_index INT NOT NULL CHECK (chunk_index>0),
 chunk_text TEXT NOT NULL,
 index_status ENUM('pending','ready','failed') NOT NULL DEFAULT 'pending',
 UNIQUE KEY rag_entry_position (knowledge_entry_id,chunk_index),
 INDEX rag_chunk_status (class_id,index_status),
 FOREIGN KEY (knowledge_entry_id,material_id,class_id) REFERENCES knowledge_entries(id,material_id,class_id),
 FULLTEXT KEY rag_chunk_fulltext (chunk_text) WITH PARSER ngram
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;
ALTER TABLE assistants ADD COLUMN class_id BIGINT NULL;
ALTER TABLE assistants ADD CONSTRAINT rag_assistant_class FOREIGN KEY (class_id) REFERENCES classes(id);
ALTER TABLE assistants ADD COLUMN prompt LONGTEXT NULL;
ALTER TABLE assistants ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE assistants ADD COLUMN revision BIGINT NOT NULL DEFAULT 0;
ALTER TABLE assistants ADD COLUMN updated_by BIGINT NULL;
ALTER TABLE assistants ADD COLUMN updated_at VARCHAR(32) NULL;
ALTER TABLE assistants ADD COLUMN skill_ids JSON NULL;
ALTER TABLE assistants ADD COLUMN material_ids JSON NULL;
ALTER TABLE assistants ADD UNIQUE rag_assistant_scope (id,class_id);
ALTER TABLE assistants ADD UNIQUE rag_assistant_name (class_id,name(255));
ALTER TABLE skills ADD COLUMN class_id BIGINT NULL;
ALTER TABLE skills ADD CONSTRAINT rag_skill_class FOREIGN KEY (class_id) REFERENCES classes(id);
ALTER TABLE skills ADD COLUMN skill_key VARCHAR(64) NULL;
ALTER TABLE skills ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE skills ADD COLUMN instruction_version VARCHAR(64) NULL;
ALTER TABLE skills ADD COLUMN revision BIGINT NOT NULL DEFAULT 0;
ALTER TABLE skills ADD COLUMN updated_by BIGINT NULL;
ALTER TABLE skills ADD COLUMN updated_at VARCHAR(32) NULL;
ALTER TABLE skills ADD UNIQUE rag_skill_key (class_id,skill_key);
CREATE TABLE IF NOT EXISTS conversations (
 id BIGINT PRIMARY KEY AUTO_INCREMENT,
 user_id BIGINT NOT NULL,
 class_id BIGINT NOT NULL,
 agent_id BIGINT NOT NULL,
 title VARCHAR(255) NOT NULL,
 created_at VARCHAR(32) NOT NULL,
 updated_at VARCHAR(32) NOT NULL,
 active_turn_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 UNIQUE KEY rag_conversation_scope(id,class_id),
 INDEX rag_owner_updated(user_id,class_id,updated_at,id),
 FOREIGN KEY(user_id) REFERENCES users(id),
 FOREIGN KEY(class_id) REFERENCES classes(id),
 FOREIGN KEY(agent_id,class_id) REFERENCES assistants(id,class_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;
CREATE TABLE IF NOT EXISTS conversation_messages (
 id BIGINT PRIMARY KEY AUTO_INCREMENT,
 conversation_id BIGINT NOT NULL,
 class_id BIGINT NOT NULL,
 turn_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 sequence_number BIGINT NOT NULL,
 role ENUM('user','assistant') NOT NULL,
 content LONGTEXT NOT NULL,
 status ENUM('generating','completed','failed','interrupted') NOT NULL,
 result_status VARCHAR(32) NULL,
 created_at VARCHAR(32) NOT NULL,
 prompt_snapshot LONGTEXT NULL,
 prompt_revision BIGINT NULL,
 configuration_snapshot JSON NULL,
 selected_skill_id BIGINT NULL,
 skill_id BIGINT NULL,
 skill_name VARCHAR(255) NOT NULL DEFAULT '',
 skill_version VARCHAR(64) NULL,
 skill_hash CHAR(64) NULL,
 selection_reason VARCHAR(64) NULL,
 evidence_snapshot JSON NULL,
 citations JSON NULL,
 failure_reason VARCHAR(64) NULL,
 UNIQUE KEY rag_message_order(conversation_id,sequence_number),
 FOREIGN KEY(conversation_id,class_id) REFERENCES conversations(id,class_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;
