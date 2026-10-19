ALTER TABLE schema_upgrades ADD COLUMN ddl_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL;
CREATE TABLE mcp_servers (
 id BIGINT AUTO_INCREMENT PRIMARY KEY,
 class_id BIGINT NOT NULL,
 name VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 base_url VARCHAR(512) NOT NULL,
 token_cipher BLOB NULL,
 token_nonce VARBINARY(12) NULL,
 key_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
 connected BOOLEAN NOT NULL DEFAULT FALSE,
 connected_at VARCHAR(32) NULL,
 created_at VARCHAR(32) NOT NULL,
 updated_at VARCHAR(32) NOT NULL,
 revision BIGINT NOT NULL DEFAULT 1,
 op_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
 operation_started_at VARCHAR(32) NULL,
 UNIQUE mcp_server_name(class_id,name),
 UNIQUE mcp_server_scope(id,class_id),
 CONSTRAINT mcp_server_class FOREIGN KEY(class_id) REFERENCES classes(id) ON DELETE RESTRICT,
 CONSTRAINT mcp_connected CHECK(connected IN(0,1)),
 CONSTRAINT mcp_credential CHECK((token_cipher IS NULL AND token_nonce IS NULL AND key_id IS NULL) OR (token_cipher IS NOT NULL AND token_nonce IS NOT NULL AND key_id IS NOT NULL))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;
CREATE TABLE mcp_tools (
 id BIGINT AUTO_INCREMENT PRIMARY KEY,
 server_id BIGINT NOT NULL,
 class_id BIGINT NOT NULL,
 tool_name VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 description TEXT NOT NULL,
 schema_json JSON NOT NULL,
 UNIQUE mcp_tool_name(server_id,tool_name),
 CONSTRAINT mcp_tool_server FOREIGN KEY(server_id,class_id) REFERENCES mcp_servers(id,class_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;
CREATE TABLE mcp_tool_calls (
 id BIGINT AUTO_INCREMENT PRIMARY KEY,
 class_id BIGINT NOT NULL,
 server_id BIGINT NOT NULL,
 user_id BIGINT NOT NULL,
 conversation_id BIGINT NULL,
 tool_ref VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 tool_name VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 server_name VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 status VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 detail VARCHAR(512) NOT NULL,
 created_at VARCHAR(32) NOT NULL,
 CONSTRAINT mcp_call_class FOREIGN KEY(class_id) REFERENCES classes(id) ON DELETE RESTRICT,
 CONSTRAINT mcp_call_user FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE RESTRICT,
 CONSTRAINT mcp_call_server FOREIGN KEY(server_id,class_id) REFERENCES mcp_servers(id,class_id) ON DELETE RESTRICT,
 CONSTRAINT mcp_call_status CHECK(status IN('ok','denied','error')),
 INDEX mcp_call_recent(class_id,created_at,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;
