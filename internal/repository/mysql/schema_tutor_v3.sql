CREATE TABLE schema_upgrades (
 version INT PRIMARY KEY,
 ddl_step INT NOT NULL,
 completed BOOLEAN NOT NULL DEFAULT FALSE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;
ALTER TABLE skills ADD COLUMN source LONGTEXT NULL;
ALTER TABLE skills ADD COLUMN trigger_text VARCHAR(120) NULL;
ALTER TABLE skills ADD COLUMN subject VARCHAR(32) NULL;
ALTER TABLE skills ADD COLUMN tools JSON NULL;
ALTER TABLE skills ADD COLUMN description TEXT NULL;
ALTER TABLE skills ADD COLUMN source_origin VARCHAR(32) NULL;
ALTER TABLE skills ADD COLUMN source_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE skills ADD COLUMN source_hash CHAR(64) NULL;
ALTER TABLE skills ADD UNIQUE tutor_skill_scope (id,class_id);
ALTER TABLE skills ADD UNIQUE tutor_skill_name (class_id,name(32));
CREATE TABLE assistant_skill_bindings (
 agent_id BIGINT NOT NULL,
 class_id BIGINT NOT NULL,
 skill_id BIGINT NOT NULL,
 PRIMARY KEY(agent_id,skill_id),
 CONSTRAINT tutor_skill_agent FOREIGN KEY(agent_id,class_id) REFERENCES assistants(id,class_id) ON DELETE CASCADE,
 CONSTRAINT tutor_skill_resource FOREIGN KEY(skill_id,class_id) REFERENCES skills(id,class_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;
CREATE TABLE assistant_material_bindings (
 agent_id BIGINT NOT NULL,
 class_id BIGINT NOT NULL,
 material_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 PRIMARY KEY(agent_id,material_id),
 CONSTRAINT tutor_material_agent FOREIGN KEY(agent_id,class_id) REFERENCES assistants(id,class_id) ON DELETE CASCADE,
 CONSTRAINT tutor_material_resource FOREIGN KEY(material_id,class_id) REFERENCES materials(id,class_id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;
