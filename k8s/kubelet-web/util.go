package main

import (
	"io"

	"github.com/emicklei/go-restful/v3"
	"github.com/olekukonko/tablewriter"
)

func AnyQueryParameter(request *restful.Request, parameters ...string) string {
	for _, parameter := range parameters {
		value := request.QueryParameter(parameter)
		if value != "" {
			return value
		}
	}
	return ""
}

func NewTableWriter(writer io.Writer) *tablewriter.Table {
	table := tablewriter.NewWriter(writer)
	table.SetBorder(false)
	table.SetHeaderLine(false)
	table.SetRowSeparator("")
	table.SetColumnSeparator("")
	table.SetCenterSeparator("")
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	return table
}

func HandleRespWithError(response *restful.Response, err error) {
	if err != nil {
		response.WriteError(500, err)
	}
}
