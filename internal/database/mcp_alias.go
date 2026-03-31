package database

// IsMcpAliasTaken devuelve true si el alias ya existe en MCP stdio o stream (excluyendo ids dados).
func IsMcpAliasTaken(alias string, excludeStdioID, excludeStreamID uint) (bool, error) {
	var n int64
	q := DB.Model(&McpStdioServer{}).Where("alias = ?", alias)
	if excludeStdioID > 0 {
		q = q.Where("id <> ?", excludeStdioID)
	}
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	q2 := DB.Model(&McpStreamServer{}).Where("alias = ?", alias)
	if excludeStreamID > 0 {
		q2 = q2.Where("id <> ?", excludeStreamID)
	}
	if err := q2.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}
