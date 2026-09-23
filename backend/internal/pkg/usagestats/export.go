package usagestats

import (
	"fmt"
	"strconv"
	"strings"
)

const MaxExportPageSize = 1000

// ExportPageOptions 仅影响导出的读取位置，用户和业务筛选仍由认证后的处理器决定。
type ExportPageOptions struct {
	BeforeID int64
	PageSize int
}

func ParseExportPageOptions(cursor, pageSize string) (ExportPageOptions, error) {
	options := ExportPageOptions{PageSize: MaxExportPageSize}
	if cursor = strings.TrimSpace(cursor); cursor != "" {
		id, err := strconv.ParseInt(cursor, 10, 64)
		if err != nil || id <= 0 {
			return options, fmt.Errorf("导出游标无效")
		}
		options.BeforeID = id
	}
	if pageSize = strings.TrimSpace(pageSize); pageSize != "" {
		size, err := strconv.Atoi(pageSize)
		if err != nil || size <= 0 || size > MaxExportPageSize {
			return options, fmt.Errorf("导出批量大小必须在 1 到 %d 之间", MaxExportPageSize)
		}
		options.PageSize = size
	}
	return options, nil
}
