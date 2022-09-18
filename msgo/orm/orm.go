package orm

import (
	"database/sql"
	"errors"
	"fmt"
	msLog "github.com/bulon99/msgo/log"
	"reflect"
	"strings"
	"time"
)

type MsDb struct {
	db     *sql.DB
	logger *msLog.Logger
	Prefix string
}

type MsSession struct {
	db           *MsDb
	tx           *sql.Tx
	beginTx      bool
	tableName    string
	fieldName    []string
	placeHolder  []string
	values       []any
	updateParam  strings.Builder
	whereParam   strings.Builder
	updateValues []any
}

func Open(driverName string, source string) *MsDb {
	db, err := sql.Open(driverName, source)
	if err != nil {
		panic(err)
	}
	//最大空闲连接数，默认不配置，是2个最大空闲连接
	db.SetMaxIdleConns(5)
	//最大连接数，默认不配置，是不限制最大连接数
	db.SetMaxOpenConns(100)
	// 连接最大存活时间
	db.SetConnMaxLifetime(time.Minute * 3)
	//空闲连接最大存活时间
	db.SetConnMaxIdleTime(time.Minute * 1)
	msDb := &MsDb{
		logger: msLog.Default(),
		db:     db,
	}
	err = db.Ping()
	if err != nil {
		panic(err)
	}
	return msDb
}

func (db *MsDb) SetMaxIdleConns(n int) {
	db.db.SetMaxIdleConns(n)
}

func (db *MsDb) New() *MsSession {
	return &MsSession{
		db: db,
	}
}

func (s *MsSession) Table(name string) *MsSession {
	s.tableName = name
	return s
}

func (s *MsSession) Insert(data any) (int64, int64, error) {
	//每一个操作是独立的，互不影响的session
	s.fieldNames(data)
	query := fmt.Sprintf("insert into %s (%s) values (%s)", s.tableName, strings.Join(s.fieldName, ","), strings.Join(s.placeHolder, ","))
	s.db.logger.Info(query)
	var stmt *sql.Stmt
	var err error
	if s.beginTx { //是否启用事务
		stmt, err = s.tx.Prepare(query)
	} else {
		stmt, err = s.db.db.Prepare(query)
	}
	if err != nil {
		return -1, -1, err
	}
	r, err := stmt.Exec(s.values...)
	if err != nil {
		return -1, -1, err
	}
	id, err := r.LastInsertId()
	if err != nil {
		return -1, -1, err
	}
	affected, err := r.RowsAffected()
	if err != nil {
		return -1, -1, err
	}
	return id, affected, nil
}

func (s *MsSession) fieldNames(data any) {
	//反射
	t := reflect.TypeOf(data)
	v := reflect.ValueOf(data)
	if v.Kind() != reflect.Pointer {
		panic(errors.New("data must be pointer"))
	}
	tVar := t.Elem()
	vVar := v.Elem()
	if s.tableName == "" { //若未定义表名，使用结构体的名称小写加前缀自动生成表名
		s.tableName = s.db.Prefix + strings.ToLower(Name(tVar.Name()))
	}
	for i := 0; i < tVar.NumField(); i++ {
		fieldName := tVar.Field(i).Name
		tag := tVar.Field(i).Tag
		sqlTag := tag.Get("msorm")
		if sqlTag == "" {
			sqlTag = strings.ToLower(Name(fieldName)) //没有标签，将struct字段转为蛇形小写，作为数据表的字段值
		} else {
			if strings.Contains(sqlTag, "auto_increment") { //自增的不用处理，跳过
				continue
			}
			if strings.Contains(sqlTag, ",") {
				sqlTag = sqlTag[:strings.Index(sqlTag, ",")] //取逗号分割的第一个内容作为数据表的字段值
			}
		}
		s.fieldName = append(s.fieldName, sqlTag)
		s.placeHolder = append(s.placeHolder, "?")
		s.values = append(s.values, vVar.Field(i).Interface())
	}
}

func Name(name string) string { //在驼峰前加下划线  将UserName --> User_Name
	preIndex := 0
	var sb strings.Builder
	for index, value := range name {
		if value >= 65 && value <= 90 { //大写字母
			if index == 0 { //首字母大写跳过
				continue
			}
			sb.WriteString(name[preIndex:index])
			sb.WriteString("_")
			preIndex = index
		}
	}
	sb.WriteString(name[preIndex:])
	return sb.String()
}

func (s *MsSession) BatchInsert(data []any) (int64, int64, error) {
	if len(data) == 0 {
		return -1, -1, errors.New("no data insert")
	}
	//批量插入 insert into table (x,x) values (),()
	s.batchFieldNames(data)
	query := fmt.Sprintf("insert into %s (%s) values ", s.tableName, strings.Join(s.fieldName, ","))
	var sb strings.Builder
	sb.WriteString(query)
	for index, _ := range data {
		sb.WriteString("(")
		sb.WriteString(strings.Join(s.placeHolder, ","))
		sb.WriteString(")")
		if index < len(data)-1 { //不是最后一条记录插入逗号
			sb.WriteString(",")
		}
	}
	stmt, err := s.db.db.Prepare(sb.String())
	if err != nil {
		return -1, -1, err
	}
	r, err := stmt.Exec(s.values...)
	if err != nil {
		s.db.logger.Error(err)
		return -1, -1, err
	}
	id, err := r.LastInsertId()
	if err != nil {
		s.db.logger.Error(err)
		return -1, -1, err
	}
	affected, err := r.RowsAffected()
	if err != nil {
		s.db.logger.Error(err)
		return -1, -1, err
	}
	return id, affected, nil
}

func (s *MsSession) batchFieldNames(dataArray []any) {
	data := dataArray[0]
	t := reflect.TypeOf(data)
	if t.Kind() != reflect.Pointer {
		panic(errors.New("batch insert element type must be pointer"))
	}
	tVar := t.Elem()
	if s.tableName == "" {
		s.tableName = s.db.Prefix + strings.ToLower(Name(tVar.Name()))
	}
	for i := 0; i < tVar.NumField(); i++ {
		field := tVar.Field(i)
		sqlTag := field.Tag.Get("msorm")
		if sqlTag == "" {
			sqlTag = strings.ToLower(Name(field.Name))
		} else {
			contains := strings.Contains(sqlTag, "auto_increment")
			if contains {
				continue //自增跳过不处理
			}
			if strings.Contains(sqlTag, ",") {
				sqlTag = sqlTag[:strings.Index(sqlTag, ",")] //取逗号分割的第一个内容作为数据表的字段值
			}
		}
		s.fieldName = append(s.fieldName, sqlTag)
		s.placeHolder = append(s.placeHolder, "?")
	}
	for _, value := range dataArray {
		t = reflect.TypeOf(value)
		v := reflect.ValueOf(value)
		tVar = t.Elem()
		vVar := v.Elem()
		for i := 0; i < tVar.NumField(); i++ {
			field := tVar.Field(i)
			sqlTag := field.Tag.Get("msorm")
			contains := strings.Contains(sqlTag, "auto_increment")
			if contains {
				continue
			}
			s.values = append(s.values, vVar.Field(i).Interface())
		}
	}
}

func (s *MsSession) Update(data ...any) (int64, error) {
	//Update("age",1) or Update(user)
	size := len(data)
	if size == 0 {
		return -1, errors.New("params error")
	}
	single := true
	if size > 1 {
		single = false
	}
	if !single {
		for i, v := range data {
			if i%2 == 0 {
				s.updateParam.WriteString(v.(string))
				s.updateParam.WriteString(" = ?")
				if i != len(data)-2 {
					s.updateParam.WriteString(",")
				}
			} else {
				s.updateValues = append(s.updateValues, v)
			}
		}
	} else { //传入的是结构体
		d := data[0]
		t := reflect.TypeOf(d)
		v := reflect.ValueOf(d)
		if t.Kind() != reflect.Pointer {
			return -1, errors.New("data not pointer")
		}
		tVar := t.Elem()
		vVar := v.Elem()
		if s.tableName == "" {
			s.tableName = s.db.Prefix + strings.ToLower(Name(tVar.Name()))
		}
		for i := 0; i < tVar.NumField(); i++ {
			if s.updateParam.String() != "" {
				s.updateParam.WriteString(",")
			}
			sqlTag := tVar.Field(i).Tag.Get("msorm")
			if sqlTag == "" {
				sqlTag = strings.ToLower(Name(tVar.Field(i).Name))
			} else {
				if strings.Contains(sqlTag, "auto_increment") { //自增的不用处理，跳过
					continue
				}
				if strings.Contains(sqlTag, ",") {
					sqlTag = sqlTag[:strings.Index(sqlTag, ",")]
				}
			}
			fieldValue := vVar.Field(i).Interface()
			s.updateParam.WriteString(sqlTag)
			s.updateParam.WriteString(" = ?")
			s.updateValues = append(s.updateValues, fieldValue)
		}
	}
	query := fmt.Sprintf("update %s set %s %s", s.tableName, s.updateParam.String(), s.whereParam.String())
	fmt.Println(query)
	stmt, err := s.db.db.Prepare(query)
	if err != nil {
		return -1, err
	}
	s.updateValues = append(s.updateValues, s.values...) //where条件值是添加到values的，现在合并到一起，这意味在更新时要先调用where，再调用update
	r, err := stmt.Exec(s.updateValues...)
	if err != nil {
		return -1, err
	}
	affected, err := r.RowsAffected()
	if err != nil {
		return -1, err
	}
	return affected, nil
}

func (s *MsSession) Where(field string, data any) *MsSession {
	if s.whereParam.String() == "" {
		s.whereParam.WriteString("where ")
	}
	s.whereParam.WriteString(field)
	s.whereParam.WriteString(" = ?")
	s.values = append(s.values, data)
	return s
}

func (s *MsSession) And() *MsSession {
	s.whereParam.WriteString(" and ")
	return s
}

func (s *MsSession) Or() *MsSession {
	s.whereParam.WriteString(" or ")
	return s
}

//查询单个
func (s *MsSession) SelectOne(data any, fields ...string) error {
	t := reflect.TypeOf(data)
	v := reflect.ValueOf(data)
	var fieldStr = "*"
	if len(fields) > 0 {
		fieldStr = strings.Join(fields, ",")
	}
	if t.Kind() != reflect.Pointer {
		panic(errors.New("data type must be pointer"))
	}
	if s.tableName == "" {
		s.tableName = s.db.Prefix + strings.ToLower(Name(t.Elem().Name()))
	}
	query := fmt.Sprintf("select %s from %s ", fieldStr, s.tableName)
	var sb strings.Builder
	sb.WriteString(query)
	sb.WriteString(s.whereParam.String())
	s.db.logger.Info(sb.String())
	stmt, err := s.db.db.Prepare(sb.String())
	if err != nil {
		return err
	}
	rows, err := stmt.Query(s.values...)
	if err != nil {
		return err
	}
	columns, err := rows.Columns() //表字段切片
	if err != nil {
		return err
	}
	values := make([]any, len(columns))
	fieldsScan := make([]any, len(columns))
	for i := range fieldsScan {
		fieldsScan[i] = &values[i] //这里很巧妙，fieldsScan存储地址，该地址指向values对应位置的值
	}
	if rows.Next() {
		err = rows.Scan(fieldsScan...) //相当于把查询的值扫描到values
		if err != nil {
			return err
		}
		valueOf := reflect.ValueOf(values)
		for i := 0; i < t.Elem().NumField(); i++ { //遍历结构体中每个字段
			name := t.Elem().Field(i).Name
			tag := t.Elem().Field(i).Tag
			sqlTag := tag.Get("msorm")
			if sqlTag == "" {
				sqlTag = strings.ToLower(Name(name))
			} else {
				if strings.Contains(sqlTag, ",") {
					sqlTag = sqlTag[:strings.Index(sqlTag, ",")]
				}
			}
			for j, coName := range columns { //遍历表中每个字段
				if sqlTag == coName { //若结构体字段的标签和数据表的字段相等
					if v.Elem().Field(i).CanSet() { //结构体字段值可设置
						covertValue := s.ConvertType(valueOf, v, i, j) //将values中元素的reflect.Value类型转为结构体中对应字段的reflect.Value类型
						v.Elem().Field(i).Set(covertValue)             //然后才可以设定值
					}
					break
				}
			}
		}
	}
	return nil
}

func (s *MsSession) ConvertType(valueOf, v reflect.Value, i, j int) reflect.Value {
	val := valueOf.Index(j) //values[j]
	valValue := reflect.ValueOf(val.Interface())
	t := v.Elem().Field(i).Type()
	covertValue := valValue.Convert(t)
	return covertValue
}

//查询多个
func (s *MsSession) Select(data any, fields ...string) ([]any, error) {
	var fieldStr = "*"
	if len(fields) > 0 {
		fieldStr = strings.Join(fields, ",")
	}
	t := reflect.TypeOf(data)
	if t.Kind() != reflect.Pointer {
		panic(errors.New("data type must be struct"))
	}
	if s.tableName == "" {
		s.tableName = s.db.Prefix + strings.ToLower(Name(t.Elem().Name()))
	}
	query := fmt.Sprintf("select %s from %s ", fieldStr, s.tableName)
	var sb strings.Builder
	sb.WriteString(query)
	sb.WriteString(s.whereParam.String())
	s.db.logger.Info(sb.String())
	stmt, err := s.db.db.Prepare(sb.String())
	if err != nil {
		return nil, err
	}
	rows, err := stmt.Query(s.values...)
	if err != nil {
		return nil, err
	}
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	values := make([]any, len(columns))
	var fieldsScan = make([]any, len(columns))
	for i := range fieldsScan {
		fieldsScan[i] = &values[i]
	}
	var results []any
	for {
		if rows.Next() {
			//由于data是一个指针如果每次赋值，会导致append到result里的数据都一样，使用reflect.New
			data = reflect.New(t.Elem()).Interface()
			err = rows.Scan(fieldsScan...)
			if err != nil {
				return nil, err
			}
			v := reflect.ValueOf(data)
			valueOf := reflect.ValueOf(values)
			for i := 0; i < t.Elem().NumField(); i++ {
				name := t.Elem().Field(i).Name
				tag := t.Elem().Field(i).Tag
				sqlTag := tag.Get("msorm")
				if sqlTag == "" {
					sqlTag = strings.ToLower(Name(name))
				} else {
					if strings.Contains(sqlTag, ",") {
						sqlTag = sqlTag[:strings.Index(sqlTag, ",")]
					}
				}
				for j, coName := range columns {
					if sqlTag == coName {
						if v.Elem().Field(i).CanSet() {
							covertValue := s.ConvertType(valueOf, v, i, j) //将values中元素的reflect.Value类型转为结构体中对应字段的reflect.Value类型
							v.Elem().Field(i).Set(covertValue)             //然后才可以设定值
						}
					}
				}
			}
			results = append(results, data)
		} else {
			break
		}
	}
	return results, nil
}

func (s *MsSession) Delete() (int64, error) {
	query := fmt.Sprintf("delete from %s ", s.tableName)
	var sb strings.Builder
	sb.WriteString(query)
	sb.WriteString(s.whereParam.String())
	stmt, err := s.db.db.Prepare(sb.String())
	if err != nil {
		return -1, err
	}
	result, err := stmt.Exec(s.values...)
	if err != nil {
		return -1, err
	}
	r, err := result.RowsAffected()
	if err != nil {
		return -1, err
	}
	return r, nil
}

func (s *MsSession) Like(field string, data any) *MsSession {
	if s.whereParam.String() == "" {
		s.whereParam.WriteString("where ")
	}
	s.whereParam.WriteString(field)
	s.whereParam.WriteString(" like ?")
	s.values = append(s.values, "%"+data.(string)+"%")
	return s
}

func (s *MsSession) LikeRight(field string, data any) *MsSession {
	if s.whereParam.String() == "" {
		s.whereParam.WriteString("where ")
	}
	s.whereParam.WriteString(field)
	s.whereParam.WriteString(" like ?")
	s.values = append(s.values, data.(string)+"%")
	return s
}

func (s *MsSession) Group(field ...string) *MsSession {
	s.whereParam.WriteString(" group by ")
	s.whereParam.WriteString(strings.Join(field, ","))
	return s
}

func (s *MsSession) OrderDesc(field ...string) *MsSession {
	s.whereParam.WriteString(" order by ")
	s.whereParam.WriteString(strings.Join(field, ","))
	s.whereParam.WriteString(" desc ")
	return s
}

func (s *MsSession) OrderAsc(field ...string) *MsSession {
	s.whereParam.WriteString(" order by ")
	s.whereParam.WriteString(strings.Join(field, ","))
	s.whereParam.WriteString(" asc ")
	return s
}

//order by aa asc, bb desc
func (s *MsSession) Order(field ...string) *MsSession {
	s.whereParam.WriteString(" order by ")
	size := len(field)
	if size%2 != 0 {
		panic("Order field must be even number")
	}
	for index, v := range field {
		s.whereParam.WriteString(" ")
		s.whereParam.WriteString(v)
		s.whereParam.WriteString(" ")
		if index%2 != 0 && index < len(field)-1 {
			s.whereParam.WriteString(",")
		}
	}
	return s
}

//count sum avg
func (s *MsSession) Aggregate(funcName, field string) (int64, error) {
	var aggSb strings.Builder
	aggSb.WriteString(funcName)
	aggSb.WriteString("(")
	aggSb.WriteString(field)
	aggSb.WriteString(")")
	query := fmt.Sprintf("select %s from %s ", aggSb.String(), s.tableName)
	var sb strings.Builder
	sb.WriteString(query)
	sb.WriteString(s.whereParam.String())
	s.db.logger.Info(sb.String())
	stmt, err := s.db.db.Prepare(sb.String())
	if err != nil {
		return 0, err
	}
	var result int64
	row := stmt.QueryRow()
	err = row.Err()
	if err != nil {
		return 0, err
	}
	err = row.Scan(&result)
	if err != nil {
		return 0, err
	}
	return result, nil
}

//原生sql
func (s *MsSession) Exec(sql string, values ...any) (int64, error) {
	stmt, err := s.db.db.Prepare(sql)
	if err != nil {
		return 0, err
	}
	r, err := stmt.Exec(values...)
	if err != nil {
		return 0, err
	}
	if strings.Contains(strings.ToLower(sql), "insert") {
		return r.LastInsertId()
	}
	return r.RowsAffected()
}

//事务
func (s *MsSession) Begin() error {
	tx, err := s.db.db.Begin()
	if err != nil {
		return err
	}
	s.tx = tx // 这样在事务中执行s.tx.Prepare(query)
	s.beginTx = true
	return nil
}

func (s *MsSession) Commit() error {
	err := s.tx.Commit()
	if err != nil {
		return err
	}
	s.beginTx = false
	return nil
}

func (s *MsSession) Rollback() error {
	err := s.tx.Rollback()
	if err != nil {
		return err
	}
	s.beginTx = false
	return nil
}
