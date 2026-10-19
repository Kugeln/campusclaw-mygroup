package tutor

import "strings"

const MigrationTrigger = "方程,去括号,分数,通分,面积,应用题,未知数,行程"

// LegacySkillSource synthesizes migration text, never a teacher's original file.
// Historical instruction version/hash fields must remain unchanged in storage.
func LegacySkillSource(name string, enabled bool) (SkillText, error) {
	flag := "否"
	if enabled {
		flag = "是"
	}
	body := strings.Replace(instructions, "只依据本轮合法本班证据，", "", 1)
	return ParseSkillSource("# 系统迁移生成（v3），非教师原文\n名称：" + name + "\n适用条件：" + MigrationTrigger + "\n资料：\n工具：knowledge_search\n启用：" + flag + "\n\n" + body)
}
