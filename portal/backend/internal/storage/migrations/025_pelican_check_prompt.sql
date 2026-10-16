ALTER TABLE portal_pelican_check_config ADD COLUMN IF NOT EXISTS model TEXT NOT NULL DEFAULT 'gpt-6-astra';
ALTER TABLE portal_pelican_check_config ADD COLUMN IF NOT EXISTS prompt TEXT NOT NULL DEFAULT '生成 html，内容是 svg 绘制鹈鹕骑自行车 2D 动画，不用进行测试';
