package generator

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"time"
)

type Generator struct {
	dsn     string
	project string
}

func NewGenerator() *Generator {
	return &Generator{}
}
func (gen *Generator) Dsn(dsn string) *Generator {
	gen.dsn = dsn
	return gen
}
func (gen *Generator) Project(project string) *Generator {
	gen.project = project
	return gen
}

type Module struct {
	TableName           string //表名
	TableNameUpperCamel string //表名的大驼峰
	TableNameLowerCamel string //表名的小驼峰
	ModulePath          string //模块名用于生成文件名
	Fields              []Field
	PrimaryKeyFields    []Field //主键
	Model               bool
	ModelFilePath       string //全路径，不包含文件名
	ModelFileName       string //只有文件名
	ModelPackageName    string //只有包名，不包含文件名
	ModelPackagePath    string //包含完整的包名

	Extend            bool
	ExtendFilePath    string //全路径，不包含文件名
	ExtendFileName    string //只有文件名
	ExtendPackageName string //只有包名，不包含文件名
	ExtendPackagePath string //包含完整的包名

	View            bool
	ViewFilePath    string
	ViewFileName    string
	ViewPackageName string
	ViewPackagePath string

	Param            bool
	ParamFilePath    string
	ParamFileName    string
	ParamPackageName string
	ParamPackagePath string

	Dao            bool
	DaoFilePath    string
	DaoFileName    string
	DaoPackageName string
	DaoPackagePath string

	Service            bool
	ServiceFilePath    string
	ServiceFileName    string
	ServicePackageName string
	ServicePackagePath string

	Controller            bool
	ControllerFilePath    string
	ControllerFileName    string
	ControllerPackageName string
	ControllerPackagePath string

	// UpdateSql          string
	// UpdateSelectiveSql string
	CreateTime string
}

type Field struct {
	ColumnName           string         //msyql字段名 user_id
	ColumnNameLowerCamel string         //小驼峰 userId
	ColumnNameUpper      string         //字段名大写 USER_ID
	ColumnType           string         //msql 类型 varchat
	ColumnDefault        sql.NullString //默认值
	IsNullable           int            //允许为空
	IsPrimaryKey         int            //是否主键
	FieldName            string         //实体名称 大驼峰  UserId
	FieldNullType        string         //实体golang Null类型 sql.NullString
	FieldNullTypeValue   string         //实体golang Null类型 取值  String
	FieldType            string         //golang 类型  string
	FieldTypeDefault     string         //golang 类型  的默认值
	FieldOrmTag          string         //tag orm:
	FieldJsonTag         string         //tag json
	FieldFormTag         string         //tag form
	FieldDefaultTag      string         //tag 默认值
	Comment              string         //表中字段注释
}

var dbType = map[string]goType{
	"int":                {"int32", "sql.NullInt32", "Int32", "0"},
	"integer":            {"int32", "sql.NullInt32", "Int32", "0"},
	"tinyint":            {"int32", "sql.NullInt32", "Int32", "0"},
	"smallint":           {"int32", "sql.NullInt32", "Int32", "0"},
	"mediumint":          {"int32", "sql.NullInt32", "Int32", "0"},
	"bigint":             {"int32", "sql.NullInt32", "Int32", "0"},
	"int unsigned":       {"int32", "sql.NullInt32", "Int32", "0"},
	"integer unsigned":   {"int32", "sql.NullInt32", "Int32", "0"},
	"tinyint unsigned":   {"int32", "sql.NullInt32", "Int32", "0"},
	"smallint unsigned":  {"int32", "sql.NullInt32", "Int32", "0"},
	"mediumint unsigned": {"int32", "sql.NullInt32", "Int32", "0"},
	"bigint unsigned":    {"int32", "sql.NullInt32", "Int32", "0"},
	"bit":                {"int32", "sql.NullInt32", "Int32", "0"},
	"bool":               {"bool", "sql.NullBool", "Bool", "false"},
	"enum":               {"string", "sql.NullString", "String", "\"\""},
	"set":                {"string", "sql.NullString", "String", "\"\""},
	"varchar":            {"string", "sql.NullString", "String", "\"\""},
	"char":               {"string", "sql.NullString", "String", "\"\""},
	"tinytext":           {"string", "sql.NullString", "String", "\"\""},
	"mediumtext":         {"string", "sql.NullString", "String", "\"\""},
	"text":               {"string", "sql.NullString", "String", "\"\""},
	"longtext":           {"string", "sql.NullString", "String", "\"\""},
	"blob":               {"string", "sql.NullString", "String", "\"\""},
	"tinyblob":           {"string", "sql.NullString", "String", "\"\""},
	"mediumblob":         {"string", "sql.NullString", "String", "\"\""},
	"longblob":           {"string", "sql.NullString", "String", "\"\""},
	"date":               {"time.Time", "sql.NullTime", "Time", "nil"},
	"datetime":           {"time.Time", "sql.NullTime", "Time", "nil"},
	"timestamp":          {"time.Time", "sql.NullTime", "Time", "nil"},
	"time":               {"time.Time", "sql.NullTime", "Time", "nil"},
	"float":              {"float64", "sql.NullFloat64", "Float64", "0"},
	"double":             {"float64", "sql.NullFloat64", "Float64", "0"},
	"decimal":            {"float64", "sql.NullFloat64", "Float64", "0"},
	"binary":             {"string", "sql.NullString", "String", "\"\""},
	"varbinary":          {"string", "sql.NullString", "String", "\"\""},
}

type goType struct {
	baseType      string
	nullType      string
	nullTypeValue string
	defaultValue  string
}

func getFields(tableName string, db *sql.DB) ([]Field, []Field, error) {
	var sqlStr = `select
					column_name name,
					data_type type,
					if('YES'=is_nullable,true,false) isNullable,
					if('PRI'=column_key,true,false) isPrimaryKey,
					column_comment comment,column_default 'default'
				from
					information_schema.COLUMNS 
				where
					table_schema = DATABASE() `
	sqlStr += fmt.Sprintf(" and table_name = '%s' order by isPrimaryKey desc", tableName)
	rows, err := db.Query(sqlStr)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	fields := make([]Field, 0)
	primaryKeyFields := make([]Field, 0)
	for rows.Next() {
		field := Field{}
		err = rows.Scan(&field.ColumnName, &field.ColumnType, &field.IsNullable, &field.IsPrimaryKey, &field.Comment, &field.ColumnDefault)
		if err != nil {
			panic(err)
		}
		field.FieldName = ToUpperCamelCase(field.ColumnName)
		field.ColumnNameLowerCamel = ToLowerCamelCase(field.ColumnName)
		field.ColumnNameUpper = strings.ToUpper(field.ColumnName)
		field.FieldType = dbType[field.ColumnType].baseType
		field.FieldTypeDefault = dbType[field.ColumnType].defaultValue
		field.FieldNullType = dbType[field.ColumnType].nullType
		field.FieldNullTypeValue = dbType[field.ColumnType].nullTypeValue
		field.FieldOrmTag = fmt.Sprintf("orm:\"%v\"", field.ColumnName)
		field.FieldJsonTag = fmt.Sprintf("json:\"%v\"", field.ColumnName)
		field.FieldFormTag = fmt.Sprintf("form:\"%v\"", field.ColumnName)
		val, _ := field.ColumnDefault.Value()
		if val != nil {
			field.FieldDefaultTag = fmt.Sprintf("default:\"%v\"", val)
		}
		if field.IsPrimaryKey == 1 {
			primaryKeyFields = append(primaryKeyFields, field)
		}
		fields = append(fields, field)
	}
	return fields, primaryKeyFields, nil
}

func (gen *Generator) Gen(modules []Module) error {
	if gen.project == "" {
		return errors.New("project can not nil")
	}
	if gen.dsn == "" {
		return errors.New("dsn can not nil")
	}
	db, err := sql.Open("mysql", gen.dsn)
	if err != nil {
		return err
	}
	if modules == nil {
		return errors.New("modules can not nil")
	}

	for _, module := range modules {
		fields, primaryKeyFields, err := getFields(module.TableName, db)
		if err != nil {
			fmt.Printf("error:create table %v error=%v", module.TableName, err)
			continue
		}
		if primaryKeyFields == nil || len(primaryKeyFields) == 0 {
			fmt.Printf("error:table %v no primary key", module.TableName)
			continue
		}
		tableName := module.TableName
		module.CreateTime = time.Now().Format("2006-01:02 15:04:05.006")
		module.Fields = fields
		module.PrimaryKeyFields = primaryKeyFields
		module.TableNameUpperCamel = ToUpperCamelCase(tableName)
		module.TableNameLowerCamel = ToLowerCamelCase(tableName)
		urls := strings.Split(module.ModulePath, gen.project)

		if module.ModelPackageName == "" {
			module.ModelPackageName = "model"
		}

		module.ModelPackagePath = gen.project + urls[1] + "/" + module.ModelPackageName
		module.ModelFileName = tableName + "_" + module.ModelPackageName + ".go"
		module.ModelFilePath = module.ModulePath + "/" + module.ModelPackageName

		module.ExtendPackageName = "extend"
		module.ExtendPackagePath = gen.project + urls[1] + "/" + module.ModelPackageName
		module.ExtendFileName = tableName + "_" + module.ExtendPackageName + ".go"
		module.ExtendFilePath = module.ModulePath + "/" + module.ModelPackageName

		module.ViewPackageName = "view"
		module.ViewPackagePath = gen.project + urls[1] + "/" + module.ViewPackageName
		module.ViewFileName = tableName + "_" + module.ViewPackageName + ".go"
		module.ViewFilePath = module.ModulePath + "/" + module.ViewPackageName

		module.ParamPackageName = "param"
		module.ParamPackagePath = gen.project + urls[1] + "/" + module.ParamPackageName
		module.ParamFileName = tableName + "_" + module.ParamPackageName + ".go"
		module.ParamFilePath = module.ModulePath + "/" + module.ParamPackageName

		module.DaoPackageName = "dao"
		module.DaoPackagePath = gen.project + urls[1] + "/" + module.DaoPackageName
		module.DaoFileName = tableName + "_" + module.DaoPackageName + ".go"
		module.DaoFilePath = module.ModulePath + "/" + module.DaoPackageName

		module.ServicePackageName = "service"
		module.ServicePackagePath = gen.project + urls[1] + "/" + module.ServicePackageName
		module.ServiceFileName = tableName + "_" + module.ServicePackageName + ".go"
		module.ServiceFilePath = module.ModulePath + "/" + module.ServicePackageName

		module.ControllerPackageName = "controller"
		module.ControllerPackagePath = gen.project + urls[1] + "/" + module.ControllerPackageName
		module.ControllerFileName = tableName + "_" + module.ControllerPackageName + ".go"
		module.ControllerFilePath = module.ModulePath + "/" + module.ControllerPackageName

		if module.Model {
			genFile(&module, module.ModelPackageName)
		}
		if module.Extend {
			genFile(&module, module.ExtendPackageName)
		}
		if module.View {
			genFile(&module, module.ViewPackageName)
		}
		if module.Param {
			genFile(&module, module.ParamPackageName)
		}
		if module.Dao {
			genFile(&module, module.DaoPackageName)
		}
		if module.Service {
			genFile(&module, module.ServicePackageName)
		}
		if module.Controller {
			genFile(&module, module.ControllerPackageName)
		}

	}
	return nil
}

func genFile(table *Module, packageName string) {

	var templateStr, filePath, file string
	if "model" == packageName {
		templateStr = getModelTemplate()
		filePath = table.ModelFilePath
		file = filePath + "/" + table.ModelFileName
	} else if "extend" == packageName {
		templateStr = getExtendTemplate()
		filePath = table.ExtendFilePath
		file = filePath + "/" + table.ExtendFileName
		if IsExist(file) { //extend 不覆盖
			return
		}
	} else if "view" == packageName {
		templateStr = getViewTemplate()
		filePath = table.ViewFilePath
		file = filePath + "/" + table.ViewFileName
		if IsExist(file) { //view 不覆盖
			return
		}
	} else if "param" == packageName {
		templateStr = getParamTemplate()
		filePath = table.ParamFilePath
		file = filePath + "/" + table.ParamFileName
		if IsExist(file) { //param 不覆盖
			return
		}
	} else if "dao" == packageName {
		templateStr = getDaoTemplate()
		filePath = table.DaoFilePath
		file = filePath + "/" + table.DaoFileName
		if IsExist(file) { //dao 不覆盖
			return
		}
	} else if "service" == packageName {
		templateStr = getServiceTemplate()
		filePath = table.ServiceFilePath
		file = filePath + "/" + table.ServiceFileName
		if IsExist(file) { //service 不覆盖
			return
		}
	} else if "controller" == packageName {
		templateStr = getController()
		filePath = table.ControllerFilePath
		file = filePath + "/" + table.ControllerFileName
		if IsExist(file) { //controller 不覆盖
			return
		}
	}
	// 第一步，加载模版文件
	tmpl, err := template.New("tmpl").Parse(templateStr)
	if err != nil {
		fmt.Println("create template model, err:", err)
		return
	}
	// 第二步，创建文件目录
	err = CreateDir(filePath)
	if err != nil {
		fmt.Printf("create path:%v err", filePath)
		return
	}
	// 第三步，创建且打开文件
	f, err := os.OpenFile(file, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		fmt.Println("Can not write file")
		return
	}
	defer f.Close()

	// 第四步，写入数据
	tmpl.Execute(f, table)

	//第五步，格式化代码
	cmd := exec.Command("gofmt", "-w", file)
	cmd.Run()
}
func getViewTemplate() string {
	return ` // Create by code generator  {{.CreateTime}}
	package view
	
	import (
		"{{.ModelPackagePath}}"
		"time"
	)
	type {{.TableNameUpperCamel}}View struct {
		{{range $field := .Fields}}{{ .FieldName }}  {{ .FieldType }} ` + "`{{ .FieldJsonTag }}`" + ` // {{ .Comment }}
		{{end}}
	}
	func Convert(m *model.{{.TableNameUpperCamel}}Model) *{{.TableNameUpperCamel}}View {
		return &{{.TableNameUpperCamel}}View{
			{{range $field := .Fields}}{{ .FieldName }} : m.{{ .FieldName }}.{{ .FieldNullTypeValue}},
			{{end}}
		}
	}
	func Converts(models []model.{{.TableNameUpperCamel}}Model) []{{.TableNameUpperCamel}}View {
		views := make([]{{.TableNameUpperCamel}}View, 0, len(models))
		for _, model := range models {
			views = append(views, *Convert(&model))
		}
		return views
	}
	
	func ConvertExtend(m *model.{{.TableNameUpperCamel}}Extend) *{{.TableNameUpperCamel}}View {
		view := Convert(&m.{{.TableNameUpperCamel}}Model)
		return view
	}
	func ConvertExtends(extends []model.{{.TableNameUpperCamel}}Extend) []{{.TableNameUpperCamel}}View {
		views := make([]{{.TableNameUpperCamel}}View, 0, len(extends))
		for _, extend := range extends {
			views = append(views, *ConvertExtend(&extend))
		}
		return views
	}`
}
func getServiceTemplate() string {
	return `// Create by code generator  {{.CreateTime}}
			package service

			import (
				"{{.DaoPackagePath}}"
				"{{.ModelPackagePath}}"
				"{{.ParamPackagePath}}"
			
				generator "github.com/go-lazyer/go-generator/sql"
			)


			func QueryByPrimaryKey({{range $i,$field := .PrimaryKeyFields}} {{if ne $i 0}},{{end}}{{ .ColumnNameLowerCamel }} interface{}  {{end}}) (*model.{{.TableNameUpperCamel}}Model, error) {
				{{.TableNameLowerCamel}}, err := dao.QueryByPrimaryKey({{range $i,$field := .PrimaryKeyFields}} {{if ne $i 0}},{{end}}{{ .ColumnNameLowerCamel }}   {{end}})
				if err != nil {
					return nil,err
				}
				return {{.TableNameLowerCamel}},nil
			}


			func QueryByParam({{.TableNameLowerCamel}}Param *param.{{.TableNameUpperCamel}}Param) ([]model.{{.TableNameUpperCamel}}Model, error) {
				query := generator.NewBoolQuery()
				gen := generator.NewGenerator().PageNum({{.TableNameLowerCamel}}Param.PageNum).PageStart({{.TableNameLowerCamel}}Param.PageStart).PageSize({{.TableNameLowerCamel}}Param.PageSize).Table(model.TABLE_NAME).Where(query)
				{{.TableNameLowerCamel}}s, err := dao.QueryByGen(gen)
				if err != nil {
					return nil,err
				}
				return {{.TableNameLowerCamel}}s,nil
			}`
}
func getParamTemplate() string {
	return `// Create by code generator  {{.CreateTime}}
			package param
			
			import (
				"time"
			)
			type {{.TableNameUpperCamel}}Param struct {
				{{range $field := .Fields}}{{ .FieldName }}  {{ .FieldType }} ` + "`{{.FieldFormTag}} {{ .FieldJsonTag }}`" + ` // {{ .Comment }}
				{{end}}
				PageNum 	int ` + "`form:\"page\" json:\"page\"`" + `
				PageStart 	int ` + "`form:\"start\" json:\"start\"`" + `
				PageSize 	int ` + "`form:\"size\" json:\"size\"`" + `
			}`
}
func getModelTemplate() string {
	return `// Create by code generator  {{.CreateTime}}
	package model
	
	import (
		"bytes"
		"database/sql"
		"errors"
		"fmt"
	)
	
	const (
		{{range $field := .Fields}}
			{{- .ColumnNameUpper -}}  ="{{ .ColumnName }}" // {{ .Comment }}
		{{end}}
		TABLE_NAME  = "{{ .TableName }}" // 表名
	)
	
	type {{.TableNameUpperCamel}}Model struct {
		{{range $field := .Fields}}{{ .FieldName }}  {{ .FieldNullType }} ` + "`{{ .FieldOrmTag }} {{ .FieldDefaultTag }}`" + ` // {{ .Comment }}
		{{end}}
	}
	
	func (m *{{.TableNameUpperCamel}}Model) format(format string, includeEmpty bool) (string, []interface{}) {
		params := make([]interface{}, 0)
		var sql bytes.Buffer

		{{$n := -1}}
        {{range $i,$field := .Fields}}
            if m.{{$field.FieldName}}.Valid {
                sql.WriteString({{if ne $i  0}}","+{{end}}fmt.Sprintf(format,"{{$field.ColumnName}}"))
                params = append(params, m.{{$field.FieldName}}.{{.FieldNullTypeValue}})
            } else if includeEmpty {
                sql.WriteString({{if ne $i  0}}","+{{end}}fmt.Sprintf(format,"{{$field.ColumnName}}"))
                params = append(params, {{if .ColumnDefault.Valid}}"{{.ColumnDefault.String}}"{{else}}nil{{end}})
            }
        {{end}}
		return sql.String(), params
	}
	func (m *{{.TableNameUpperCamel}}Model) UpdateSql() (string, []interface{}, error) {
		{{range $field := .Fields}}
			{{if eq .IsPrimaryKey 1}}
			if !m.{{ .FieldName }}.Valid {
				return "", nil, errors.New("{{ .FieldName }} is not null")
		}{{end}}{{end}}

		str, params := m.format("` + " `%v`" + ` = ?", true)
		var sql bytes.Buffer
		sql.WriteString(fmt.Sprintf("update ` + "`{{.TableName}}`" + ` set %v", str))
	
		sql.WriteString(" where  {{range $i,$field := .Fields}}{{if eq $field.IsPrimaryKey 1}}{{if ne $n  -1}} and {{end}}{{$n = $i}}` + "`{{$field.ColumnName}}`" + ` = ?{{end}}{{end}} "){{$n = -1}}
		params = append(params {{range $i,$field := .Fields}}{{if eq $field.IsPrimaryKey 1}},m.{{$field.FieldName}}.{{.FieldNullTypeValue}}{{end}}{{end}})
		return sql.String(), params, nil
	}
	
	func (m *{{.TableNameUpperCamel}}Model) UpdateSqlBySelective() (string, []interface{}, error) {
		{{range $field := .Fields}}{{if eq .IsPrimaryKey 1}} if !m.{{ .FieldName }}.Valid {
			return "", nil, errors.New("{{ .FieldName }} is not null")
		}{{end}}
		{{end}}
	
		str, params := m.format("` + " `%v`" + ` = ?", false)
		var sql bytes.Buffer
		sql.WriteString(fmt.Sprintf("update ` + "`{{.TableName}}`" + ` set %v", str))
		
		{{$n = -1}}
		sql.WriteString(" where  {{range $i,$field := .PrimaryKeyFields}}{{if ne $n  -1}} and {{end}}{{$n = $i}}` + "`{{$field.ColumnName}}`" + ` = ?{{end}} ")
		
		{{$n = -1}}
		params = append(params {{range $i,$field := .PrimaryKeyFields}},m.{{$field.FieldName}}.{{.FieldNullTypeValue}}{{end}})
		return sql.String(), params, nil
	}
	
	func (m *{{.TableNameUpperCamel}}Model) InsertSql() (string, []interface{}, error) {
		str, params := m.format("` + " `%v`" + `", true)
		var sql bytes.Buffer
		sql.WriteString(fmt.Sprintf("insert into ` + "`{{.TableName}}`" + ` (%v) values (", str))
		
		for i := 0; i < len(params); i++ {
			if i == 0 {
				sql.WriteString(" ? ")
			} else {
				sql.WriteString(",? ")
			}
		}
		sql.WriteString(")")
		
		return sql.String(), params, nil
	}

	func (m *{{.TableNameUpperCamel}}Model) SaveSql() (string, []interface{}, error) {
		insertStr, params, _ := m.InsertSql()
		var sql bytes.Buffer
		updateStr, updateParams := m.format(" ` + " `%v`" + ` = ?", true)
		sql.WriteString(fmt.Sprintf("%v on duplicate key update %v", insertStr, updateStr))
		params = append(params, updateParams...)
		return sql.String(), params, nil
	}`
}
func getExtendTemplate() string {
	return `package model

			type {{.TableNameUpperCamel}}Extend struct {
				{{.TableNameUpperCamel}}Model
			}
			`
}
func getDaoTemplate() string {
	return `// Create by go-generator  {{.CreateTime}}
			package dao
			
			import (
				dbutil "github.com/go-lazyer/go-generator/db"
				generator "github.com/go-lazyer/go-generator/sql"
				"{{.ModelPackagePath}}"
				"database/sql"
				"github.com/pkg/errors"
			)
			// query first by primaryKey
			func QueryByPrimaryKey({{range $i,$field := .PrimaryKeyFields}} {{if ne $i 0}},{{end}}{{ .ColumnNameLowerCamel }} interface{}  {{end}}) (*model.{{.TableNameUpperCamel}}Model, error) {
				{{ if eq (len .PrimaryKeyFields) 1 -}} 
				gen := generator.NewGenerator().Table(model.TABLE_NAME).Where(generator.NewEqualQuery(model.{{(index .PrimaryKeyFields 0).ColumnNameUpper}}, {{(index .PrimaryKeyFields 0).ColumnNameLowerCamel}}))
				{{ else -}}
				query := generator.NewBoolQuery(){{range $field := .PrimaryKeyFields}} .And(generator.NewEqualQuery(model.{{ .ColumnNameUpper }}, {{ .ColumnNameLowerCamel }})) {{end}}
				gen := generator.NewGenerator().Table(model.TABLE_NAME).Where(query)
				{{ end -}}
				sqlStr, params, err := gen.SelectSql(true)
				if err != nil {
					err = errors.WithStack(err)
					return nil,err
				}
				return QueryFirstBySql(sqlStr, params)
			}
			// query first by gen
			func QueryFirstByGen(gen *generator.Generator) (*model.{{.TableNameUpperCamel}}Model, error) {
				sqlStr, params, err := gen.SelectSql(true)
				if err != nil {
					err = errors.WithStack(err)
					return nil,err
				}
				return QueryFirstBySql(sqlStr, params)
			}
			// query first by sql
			func QueryFirstBySql(sqlStr string, params []interface{}) (*model.{{.TableNameUpperCamel}}Model, error) {
				var {{.TableNameLowerCamel}} model.{{.TableNameUpperCamel}}Model
				err := dbutil.PrepareFirst(sqlStr, params, &{{.TableNameLowerCamel}},&sql.DB{})
				if err != nil {
					err = errors.WithStack(err)
					return nil,err
				}
				if {{.TableNameLowerCamel}}.{{(index .PrimaryKeyFields 0).FieldName}}.{{(index .PrimaryKeyFields 0).FieldNullTypeValue}} == {{(index .PrimaryKeyFields 0).FieldTypeDefault}} {
					return nil,nil
				}
				return &{{.TableNameLowerCamel}},nil
			}
			{{if eq (len .PrimaryKeyFields) 1}} 
			// query map by primaryKeys
			func QueryMapByPrimaryKeys(primaryKeys []interface{}) (map[{{(index .PrimaryKeyFields 0).FieldType}}]model.{{.TableNameUpperCamel}}Model, error) {
				gen := generator.NewGenerator().Table(model.TABLE_NAME).Where(generator.NewInQuery(model.{{(index .PrimaryKeyFields 0).ColumnNameUpper}}, primaryKeys))
				sqlStr, params, err := gen.SelectSql(true)
				if err != nil {
					err = errors.WithStack(err)
					return nil,err
				}
				return QueryMapBySql(sqlStr, params)
			}
			{{end}}
			
			// query map by gen
			func QueryMapByGen(gen *generator.Generator) (map[{{(index .PrimaryKeyFields 0).FieldType}}]model.{{.TableNameUpperCamel}}Model, error) {
				sqlStr, params, err := gen.SelectSql(true)
				if err != nil {
					err = errors.WithStack(err)
					return nil, err
				}
				return QueryMapBySql(sqlStr, params)
			}
			
			// query map by sql
			func QueryMapBySql(sqlStr string, params []interface{}) (map[{{(index .PrimaryKeyFields 0).FieldType}}]model.{{.TableNameUpperCamel}}Model, error) {
				{{.TableNameLowerCamel}}s := make([]model.{{.TableNameUpperCamel}}Model, 0)
				err := dbutil.PrepareQuery(sqlStr, params, &{{.TableNameLowerCamel}}s,&sql.DB{})
				if err != nil {
					err = errors.WithStack(err)
					return nil,err
				}
				if {{.TableNameLowerCamel}}s == nil || len({{.TableNameLowerCamel}}s) == 0 {
					return nil,nil
				}
				{{.TableNameLowerCamel}}Map := make(map[{{(index .PrimaryKeyFields 0).FieldType}}]model.{{.TableNameUpperCamel}}Model, len({{.TableNameLowerCamel}}s))
				for _, {{.TableNameLowerCamel}} := range {{.TableNameLowerCamel}}s {
					{{.TableNameLowerCamel}}Map[{{.TableNameLowerCamel}}.{{(index .PrimaryKeyFields 0).FieldName}}.{{(index .PrimaryKeyFields 0).FieldNullTypeValue}}] = {{.TableNameLowerCamel}}
				}
				return {{.TableNameLowerCamel}}Map,nil
			}

			// count by gen
			func CountByGen(gen *generator.Generator) (int64, error) {
				sqlStr, params, err := gen.CountSql(true)
				if err != nil {
					err = errors.WithStack(err)
					return 0,err
				}
				return CountBySql(sqlStr, params)
				
			}
			// count by gen
			func CountBySql(sqlStr string, params []interface{}) (int64, error) {
				count, err := dbutil.PrepareCount(sqlStr, params,&sql.DB{})
				if err != nil {
					err = errors.WithStack(err)
					return 0,err
				}
				return count,nil
			}

			// query by gen
			func QueryByGen(gen *generator.Generator) ([]model.{{.TableNameUpperCamel}}Model, error) {
				sqlStr, params, err := gen.SelectSql(true)
				if err != nil {
					err = errors.WithStack(err)
					return nil,err
				}
				return QueryBySql(sqlStr, params)
			}
			// query by sql
			func QueryBySql(sqlStr string, params []interface{}) ([]model.{{.TableNameUpperCamel}}Model, error) {
				{{.TableNameLowerCamel}}s := make([]model.{{.TableNameUpperCamel}}Model, 0)
				err := dbutil.PrepareQuery(sqlStr, params, &{{.TableNameLowerCamel}}s,&sql.DB{})
				if err != nil {
					err = errors.WithStack(err)
					return nil,err
				}
				return {{.TableNameLowerCamel}}s,nil
			}


			// query extend by gen
			func QueryExtendByGen(gen *generator.Generator) ([]model.{{.TableNameUpperCamel}}Extend, error) {
				sqlStr, params, err := gen.SelectSql(true)
				if err != nil {
					err = errors.WithStack(err)
					return nil,err
				}
				return QueryExtendBySql(sqlStr, params)
			}
			// query extend by sql
			func QueryExtendBySql(sqlStr string, params []interface{}) ([]model.{{.TableNameUpperCamel}}Extend, error) {
				{{.TableNameLowerCamel}}Extends := make([]model.{{.TableNameUpperCamel}}Extend, 0)
				err := dbutil.PrepareQuery(sqlStr, params, &{{.TableNameLowerCamel}}Extends,&sql.DB{})
				if err != nil {
					err = errors.WithStack(err)
					return nil,err
				}
				return {{.TableNameLowerCamel}}Extends,nil
			}

			func Insert({{.TableNameLowerCamel}} *model.{{.TableNameUpperCamel}}Model) (int64, error) {
				sqlStr, params, err := {{.TableNameLowerCamel}}.InsertSql()
				if err != nil {
					err = errors.WithStack(err)
					return 0,err
				}
				return InsertBySql(sqlStr, params)
			}
			
			func InsertByGen(gen *generator.Generator) (int64, error) {
				sqlStr, params, err := gen.InsertSql(true)
				if err != nil {
					err = errors.WithStack(err)
					return 0, err
				}
				return InsertBySql(sqlStr, params)
			}
			
			func InsertBySql(sqlStr string, params []interface{}) (int64, error) {
				id, err := dbutil.PrepareInsert(sqlStr, params, &sql.DB{})
				if err != nil {
					err = errors.WithStack(err)
					return 0, err
				}
				return id, nil
			}

			//batch insert
			func Inserts(maps []map[string]interface{}) (int64, error) {
				gen := generator.NewGenerator().Table(model.TABLE_NAME).Inserts(maps)
				sqlStr, params, err := gen.InsertsSql(true)
				if err != nil {
					return 0, errors.WithStack(err)
				}
				return InsertsBySql(sqlStr, params)
			}

			//batch insert
			func InsertsByGen(gen *generator.Generator) (int64, error) {
				sqlStr, params, err := gen.InsertsSql(true)
				if err != nil {
					return 0, errors.WithStack(err)
				}
				return InsertsBySql(sqlStr, params)
			}

			//batch insert
			func InsertsBySql(sqlStr string, params []interface{}) (int64, error) {
				id, err := dbutil.PrepareInsert(sqlStr, params, &sql.DB{})
				if err != nil {
					return 0, errors.WithStack(err)
				}
				return id, nil
			}

			func Update({{.TableNameLowerCamel}} *model.{{.TableNameUpperCamel}}Model) (int64, error) {
				sqlStr, param, err := {{.TableNameLowerCamel}}.UpdateSql()
				if err != nil {
					return 0,err
				}
				return UpdateBySql(sqlStr, param)
			}
			func UpdateBySelective({{.TableNameLowerCamel}} *model.{{.TableNameUpperCamel}}Model) (int64, error) {
				sqlStr, param, err := {{.TableNameLowerCamel}}.UpdateSqlBySelective()
				if err != nil {
					err = errors.WithStack(err)
					return 0,err
				}
				return UpdateBySql(sqlStr, param)
			}
			func UpdateByGen(gen *generator.Generator) (int64, error) {
				sqlStr, params, err := gen.UpdateSql(true)
				if err != nil {
					err = errors.WithStack(err)
					return 0, err
				}
				return UpdateBySql(sqlStr, params)
			}
			func UpdateBySql(sqlStr string, params []interface{}) (int64, error) {
				count, err := dbutil.PrepareUpdate(sqlStr, params,&sql.DB{})
				if err != nil {
					err = errors.WithStack(err)
					return 0, err
				}
				return count, nil
			}
			//batch update
			func Updates(maps map[interface{}]map[string]interface{}) (int64, error) {
				gen := generator.NewGenerator().Table(model.TABLE_NAME).Updates(maps)
				sqlStr, params, err := gen.UpdatesSql(true)
				if err != nil {
					return 0, errors.WithStack(err)
				}
				return UpdatesBySql(sqlStr, params)
			}

			//batch update
			func UpdatesByGen(gen *generator.Generator) (int64, error) {
				sqlStr, params, err := gen.UpdatesSql(true)
				if err != nil {
					return 0, errors.WithStack(err)
				}
				return UpdatesBySql(sqlStr, params)
			}

			//batch update
			func UpdatesBySql(sqlStr string, params []interface{}) (int64, error) {
				id, err := dbutil.PrepareUpdate(sqlStr, params, &sql.DB{})
				if err != nil {
					return 0, errors.WithStack(err)
				}
				return id, nil
			}
			func Save({{.TableNameLowerCamel}} *model.{{.TableNameUpperCamel}}Model) (int64, error) {
				sqlStr, params, err := {{.TableNameLowerCamel}}.SaveSql()
				if err != nil {
					err = errors.WithStack(err)
					return 0, err
				}
				return SaveBySql(sqlStr, params)
			}
			
			func SaveBySql(sqlStr string, params []interface{}) (int64, error) {
				id, err := dbutil.PrepareSave(sqlStr, params, &sql.DB{})
				if err != nil {
					err = errors.WithStack(err)
					return 0, err
				}
				return id, nil
			}

			func DeleteByPrimaryKey({{range $i,$field := .PrimaryKeyFields}} {{if ne $i 0}},{{end}}{{ .ColumnNameLowerCamel }} interface{}  {{end}}) (int64, error) {
				{{ if eq (len .PrimaryKeyFields) 1 -}} 
				gen := generator.NewGenerator().Table(model.TABLE_NAME).Where(generator.NewEqualQuery(model.{{(index .PrimaryKeyFields 0).ColumnNameUpper}}, {{(index .PrimaryKeyFields 0).ColumnNameLowerCamel}}))
				{{ else -}}
				query := generator.NewBoolQuery(){{range $field := .PrimaryKeyFields}} .And(generator.NewEqualQuery(model.{{ .ColumnNameUpper }}, {{ .ColumnNameLowerCamel }})) {{end}}
				gen := generator.NewGenerator().Table(model.TABLE_NAME).Where(query)
				{{ end -}}
				sqlStr, params, err := gen.DeleteSql(true)
				if err != nil {
					err = errors.WithStack(err)
					return 0, err
				}
				return DeleteBySql(sqlStr, params)
			}
			func DeleteByPrimaryKeys(primaryKeys []interface{}) (int64, error) {
				gen := generator.NewGenerator().Table(model.TABLE_NAME).Where(generator.NewInQuery(model.{{(index .PrimaryKeyFields 0).ColumnNameUpper}}, primaryKeys))
				sqlStr, params, err := gen.DeleteSql(true)
				if err != nil {
					err = errors.WithStack(err)
					return 0, err
				}
				return DeleteBySql(sqlStr, params)
			}
			func DeleteByGen(gen *generator.Generator) (int64, error) {
				sqlStr, params, err := gen.DeleteSql(true)
				if err != nil {
					err = errors.WithStack(err)
					return 0, err
				}
				return DeleteBySql(sqlStr, params)
			}
			func DeleteBySql(sqlStr string, params []interface{}) (int64, error) {
				count, err := dbutil.PrepareDelete(sqlStr, params, &sql.DB{})
				if err != nil {
					err = errors.WithStack(err)
					return 0, err
				}
				return count, nil
			}`
}

func getController() string {
	return `// Create by code generator  {{.CreateTime}}
			package controller
			
			import (
				"net/http"
			
				"github.com/gin-gonic/gin"
			)
			
			func Index(g *gin.Context) {
				data := gin.H{
					"code": 200,
				}
				g.JSON(http.StatusOK, data)
			}`
}
