package adapter

import "text/template"

var modelHelperTpl = template.Must(template.New("model-helper").Parse(modelHelperTemplate))

const modelHelperTemplate = `package main

import (
    "encoding/json"
    "fmt"
    "os"
    "sort"
    "sync"

    "gorm.io/gorm/schema"
    {{- range .Imports }}
    {{ .Alias }} "{{ .Path }}"
    {{- end }}
)

var namingStrategy = schema.NamingStrategy{
    TablePrefix: {{ printf "%q" .NamingStrategy.TablePrefix }},
    SingularTable: {{ .NamingStrategy.SingularTable }},
    NoLowerCase: {{ .NamingStrategy.NoLowerCase }},
    IdentifierMaxLength: {{ .NamingStrategy.IdentifierMaxLength }},
}

type helperModelSnapshot struct {
    PackagePath string              ` + "`json:\"pkg\"`" + `
    TypeName    string              ` + "`json:\"type\"`" + `
    Table       helperTableSnapshot ` + "`json:\"table\"`" + `
}

type helperTableSnapshot struct {
    Name        string                     ` + "`json:\"name\"`" + `
    Fields      []helperFieldSnapshot      ` + "`json:\"fields\"`" + `
    Indexes     []helperIndexSnapshot      ` + "`json:\"indexes\"`" + `
    Constraints []helperConstraintSnapshot ` + "`json:\"constraints\"`" + `
}

type helperFieldSnapshot struct {
    Name                   string            ` + "`json:\"name\"`" + `
    DBName                 string            ` + "`json:\"dbName\"`" + `
    DataType               string            ` + "`json:\"dataType\"`" + `
    GORMDataType           string            ` + "`json:\"gormDataType\"`" + `
    PrimaryKey             bool              ` + "`json:\"primaryKey\"`" + `
    AutoIncrement          bool              ` + "`json:\"autoIncrement\"`" + `
    AutoIncrementIncrement int64             ` + "`json:\"autoIncrementIncrement\"`" + `
    NotNull                bool              ` + "`json:\"notNull\"`" + `
    Unique                 bool              ` + "`json:\"unique\"`" + `
    Size                   int               ` + "`json:\"size\"`" + `
    Precision              int               ` + "`json:\"precision\"`" + `
    Scale                  int               ` + "`json:\"scale\"`" + `
    HasDefaultValue        bool              ` + "`json:\"hasDefaultValue\"`" + `
    DefaultValue           string            ` + "`json:\"defaultValue\"`" + `
    Comment                string            ` + "`json:\"comment\"`" + `
    GormTag                string            ` + "`json:\"gormTag\"`" + `
    TagSettings            map[string]string ` + "`json:\"tagSettings\"`" + `
}

type helperIndexSnapshot struct {
    Name    string                     ` + "`json:\"name\"`" + `
    Class   string                     ` + "`json:\"class\"`" + `
    Type    string                     ` + "`json:\"type\"`" + `
    Where   string                     ` + "`json:\"where\"`" + `
    Comment string                     ` + "`json:\"comment\"`" + `
    Option  string                     ` + "`json:\"option\"`" + `
    Fields  []helperIndexFieldSnapshot ` + "`json:\"fields\"`" + `
}

type helperIndexFieldSnapshot struct {
    Column     string ` + "`json:\"column\"`" + `
    Expression string ` + "`json:\"expression\"`" + `
    Sort       string ` + "`json:\"sort\"`" + `
    Collate    string ` + "`json:\"collate\"`" + `
    Length     int    ` + "`json:\"length\"`" + `
    Priority   int    ` + "`json:\"priority\"`" + `
}

type helperConstraintSnapshot struct {
    Name             string   ` + "`json:\"name\"`" + `
    Type             string   ` + "`json:\"type\"`" + `
    Columns          []string ` + "`json:\"columns\"`" + `
    ReferenceTable   string   ` + "`json:\"ref_table\"`" + `
    ReferenceColumns []string ` + "`json:\"ref_columns\"`" + `
    OnUpdate         string   ` + "`json:\"on_update\"`" + `
    OnDelete         string   ` + "`json:\"on_delete\"`" + `
    Expression       string   ` + "`json:\"expression\"`" + `
}

func main() {
    cache := &sync.Map{}
    snapshots := make([]helperModelSnapshot, 0, {{ len .Targets }})
    {{- range .Targets }}
    snapshots = append(snapshots, captureSchema(cache, "{{ .PackagePath }}", "{{ .TypeName }}", new({{ .Alias }}.{{ .TypeName }})))
    {{- end }}
    sort.Slice(snapshots, func(i, j int) bool {
        if snapshots[i].Table.Name == snapshots[j].Table.Name {
            return snapshots[i].TypeName < snapshots[j].TypeName
        }
        return snapshots[i].Table.Name < snapshots[j].Table.Name
    })
    enc := json.NewEncoder(os.Stdout)
    if err := enc.Encode(snapshots); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}

func captureSchema(cache *sync.Map, pkgPath, typeName string, value interface{}) helperModelSnapshot {
    sch, err := schema.Parse(value, cache, namingStrategy)
    if err != nil {
        panic(err)
    }
    return helperModelSnapshot{
        PackagePath: pkgPath,
        TypeName:    typeName,
        Table:       encodeSchemaTable(sch),
    }
}

func encodeSchemaTable(sch *schema.Schema) helperTableSnapshot {
    fields := make([]helperFieldSnapshot, 0, len(sch.Fields))
    for _, field := range sch.Fields {
        if field.IgnoreMigration {
            continue
        }
        fields = append(fields, encodeField(field))
    }
    indexes := sch.ParseIndexes()
    indexSnaps := make([]helperIndexSnapshot, 0, len(indexes))
    for _, idx := range indexes {
        indexSnaps = append(indexSnaps, encodeIndex(idx))
    }
    constraints := encodeConstraints(sch)
    return helperTableSnapshot{
        Name:        sch.Table,
        Fields:      fields,
        Indexes:     indexSnaps,
        Constraints: constraints,
    }
}

func encodeField(field *schema.Field) helperFieldSnapshot {
    tagSettings := make(map[string]string, len(field.TagSettings))
    for k, v := range field.TagSettings {
        tagSettings[k] = v
    }
    return helperFieldSnapshot{
        Name:                   field.Name,
        DBName:                 field.DBName,
        DataType:               string(field.DataType),
        GORMDataType:           string(field.GORMDataType),
        PrimaryKey:             field.PrimaryKey,
        AutoIncrement:          field.AutoIncrement,
        AutoIncrementIncrement: field.AutoIncrementIncrement,
        NotNull:                field.NotNull,
        Unique:                 field.Unique,
        Size:                   field.Size,
        Precision:              field.Precision,
        Scale:                  field.Scale,
        HasDefaultValue:        field.HasDefaultValue,
        DefaultValue:           field.DefaultValue,
        Comment:                field.Comment,
        GormTag:                string(field.Tag),
        TagSettings:            tagSettings,
    }
}

func encodeIndex(idx *schema.Index) helperIndexSnapshot {
    fields := make([]helperIndexFieldSnapshot, 0, len(idx.Fields))
    for _, field := range idx.Fields {
        column := ""
        if field.Field != nil {
            column = field.Field.DBName
        }
        fields = append(fields, helperIndexFieldSnapshot{
            Column:     column,
            Expression: field.Expression,
            Sort:       field.Sort,
            Collate:    field.Collate,
            Length:     field.Length,
            Priority:   field.Priority,
        })
    }
    return helperIndexSnapshot{
        Name:    idx.Name,
        Class:   idx.Class,
        Type:    idx.Type,
        Where:   idx.Where,
        Comment: idx.Comment,
        Option:  idx.Option,
        Fields:  fields,
    }
}

func encodeConstraints(sch *schema.Schema) []helperConstraintSnapshot {
    seen := make(map[string]struct{})
    constraints := make([]helperConstraintSnapshot, 0)

    for name, chk := range sch.ParseCheckConstraints() {
        constraints = append(constraints, helperConstraintSnapshot{
            Name:       name,
            Type:       "CHECK",
            Columns:    []string{chk.Field.DBName},
            Expression: chk.Constraint,
        })
        seen[name] = struct{}{}
    }

    for name, uni := range sch.ParseUniqueConstraints() {
        if _, ok := seen[name]; ok {
            continue
        }
        column := ""
        if uni.Field != nil {
            column = uni.Field.DBName
        }
        constraints = append(constraints, helperConstraintSnapshot{
            Name:    name,
            Type:    "UNIQUE",
            Columns: []string{column},
        })
        seen[name] = struct{}{}
    }

    for _, rel := range sch.Relationships.Relations {
        if rel == nil || rel.Field == nil {
            continue
        }
        c := rel.ParseConstraint()
        if c == nil || c.Name == "" || c.Schema != sch {
            continue
        }
        if _, ok := seen[c.Name]; ok {
            continue
        }
        columns := make([]string, 0, len(c.ForeignKeys))
        for _, fk := range c.ForeignKeys {
            if fk != nil {
                columns = append(columns, fk.DBName)
            }
        }
        refCols := make([]string, 0, len(c.References))
        for _, ref := range c.References {
            if ref != nil {
                refCols = append(refCols, ref.DBName)
            }
        }
        refTable := ""
        if c.ReferenceSchema != nil {
            refTable = c.ReferenceSchema.Table
        }
        constraints = append(constraints, helperConstraintSnapshot{
            Name:             c.Name,
            Type:             "FOREIGN KEY",
            Columns:          columns,
            ReferenceTable:   refTable,
            ReferenceColumns: refCols,
            OnUpdate:         c.OnUpdate,
            OnDelete:         c.OnDelete,
        })
        seen[c.Name] = struct{}{}
    }

    sort.Slice(constraints, func(i, j int) bool {
        if constraints[i].Name == constraints[j].Name {
            return constraints[i].Type < constraints[j].Type
        }
        return constraints[i].Name < constraints[j].Name
    })

    return constraints
}
`
