-- Добавляем колонки для периодичности
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS recurrence_type VARCHAR(50);
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS recurrence_rule JSONB;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS is_template BOOLEAN DEFAULT FALSE;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS parent_template_id BIGINT;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS scheduled_at TIMESTAMP;

-- Добавляем внешний ключ
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_parent_template') THEN
        ALTER TABLE tasks ADD CONSTRAINT fk_parent_template 
        FOREIGN KEY (parent_template_id) REFERENCES tasks(id) ON DELETE SET NULL;
    END IF;
END $$;

-- Создаём индексы
CREATE INDEX IF NOT EXISTS idx_tasks_is_template ON tasks(is_template);
CREATE INDEX IF NOT EXISTS idx_tasks_parent_template ON tasks(parent_template_id);
CREATE INDEX IF NOT EXISTS idx_tasks_scheduled_at ON tasks(scheduled_at);

-- Уникальный индекс для предотвращения дубликатов
DROP INDEX IF EXISTS uniq_template_date;
CREATE UNIQUE INDEX uniq_template_date 
ON tasks (parent_template_id, scheduled_at) 
WHERE parent_template_id IS NOT NULL AND scheduled_at IS NOT NULL;