package postgres

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/linlexing/dbx/common"
	"github.com/linlexing/dbx/schema"
)

const driverName = "postgres"

type meta struct {
}

func (m *meta) IsNull() string {
	return "COALESCE"
}

func init() {
	schema.Register(driverName, new(meta))
}

//执行create table as select语句
func (m *meta) CreateTableAs(db common.DB, tableName, strSQL string, pks []string) error {
	s := fmt.Sprintf("CREATE TABLE %s as %s", tableName, strSQL)
	if _, err := db.Exec(s); err != nil {
		err = common.NewSQLError(err, s)
		log.Println(err)
		return err
	}
	s = fmt.Sprintf("ALTER TABLE %s ADD PRIMARY KEY(%s)", tableName, strings.Join(pks, ","))
	if _, err := db.Exec(s); err != nil {
		err = common.NewSQLError(err, s)
		log.Println(err)
		return err
	}
	return nil
}

func (m *meta) TableNames(db common.DB) (names []string, err error) {
	strSQL := "SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema()"
	names = []string{}
	rows, err := db.Query(strSQL)
	if err != nil {
		err = common.NewSQLError(err, strSQL)
		log.Println(err)
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err = rows.Scan(name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}

	sort.Strings(names)
	return
}

func (m *meta) TableExists(db common.DB, tabName string) (bool, error) {
	schemaName := ""
	ns := strings.Split(tabName, ".")
	tname := ""
	if len(ns) > 1 {
		schemaName = ns[0]
		tname = ns[1]
	} else {
		tname = tabName
	}
	if len(schemaName) == 0 {
		strSQL := "select current_schema()"
		if err := db.QueryRow(strSQL).Scan(&schemaName); err != nil {
			err = common.NewSQLError(err, strSQL)
			log.Println(err)
			return false, err
		}
	}

	strSQL :=
		"SELECT count(*) FROM information_schema.tables WHERE table_schema ilike :schema and table_name ilike :tname"
	var iCount int64
	if err := db.QueryRow(strSQL, schemaName, tname).Scan(iCount); err != nil {
		err = common.NewSQLError(err, strSQL, schemaName, tname)
		log.Println(err)
		return false, err
	}

	return iCount > 0, nil
}

/*
func (m *meta) ValueExpress(db DB, dataType datatype.DataType, value string) string {
	switch dataType {
	case TypeFloat, TypeInt:
		return value
	case TypeString:
		return safe.SignString(value)
	case TypeDatetime:
		if len(value) == 10 {
			return fmt.Sprintf("TO_DATE('%s','YYYY-MM-DD')", value)
		} else if len(value) == 19 {
			return fmt.Sprintf("TO_DATE('%s','YYYY-MM-DD HH24:MI:SS')", value)
		} else {
			panic(fmt.Errorf("invalid datetime:%s", value))
		}
	default:
		panic(fmt.Errorf("not impl ValueExpress,type:%d", dataType))
	}
}*/

func (m *meta) CreateTable(db common.DB, tab *schema.Table) error {
	cols := []string{}
	for _, v := range tab.Columns {
		cols = append(cols, dbDefine(v))
	}
	var strSQL string
	if len(tab.PrimaryKeys) > 0 {
		strSQL = fmt.Sprintf(
			"CREATE TABLE %s(\n%s,\nCONSTRAINT %s_pkey PRIMARY KEY(%s)\n)",
			tab.FullName(), strings.Join(cols, ",\n"), tab.Name, strings.Join(tab.PrimaryKeys, ","))
	} else {
		strSQL = fmt.Sprintf(
			"CREATE TABLE %s(\n%s\n)",
			tab.FullName(), strings.Join(cols, ",\n"))
	}
	if _, err := db.Exec(strSQL); err != nil {
		err = common.NewSQLError(err, strSQL)
		log.Println(err)
		return err
	}
	log.Println(strSQL)
	//最后处理索引
	for _, col := range tab.Columns {
		if col.Index {
			if err := createColumnIndex(db, tab.FullName(), col.Name); err != nil {
				return err
			}
		}
	}
	return nil
}