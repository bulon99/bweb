package service

import (
	"fmt"
	"github.com/bulon99/msgo/orm"
	_ "github.com/go-sql-driver/mysql"
	"net/url"
)

type User struct {
	Id       int64  `msorm:"id,auto_increment"`
	Username string `msorm:"username"`
	Password string `msorm:"password"`
	Age      int    `msorm:"age"`
}

func SaveUser() {
	dataSourceName := fmt.Sprintf("root:0213@tcp(localhost:3306)/msgo?charset=utf8&loc=%s&parseTime=true", url.QueryEscape("Asia/Shanghai"))
	db := orm.Open("mysql", dataSourceName)
	db.Prefix = "t_"
	user := &User{
		Username: "bulon",
		Password: "0213",
		Age:      25,
	}
	id, _, err := db.New().Insert(user)
	if err != nil {
		panic(err)
	}
	fmt.Println(id)
}

func SaveUserBatch() {
	dataSourceName := fmt.Sprintf("root:0213@tcp(localhost:3306)/msgo?charset=utf8&loc=%s&parseTime=true", url.QueryEscape("Asia/Shanghai"))
	db := orm.Open("mysql", dataSourceName)
	db.Prefix = "t_"
	user1 := &User{
		Username: "bula",
		Password: "0213",
		Age:      20,
	}
	user2 := &User{
		Username: "buhu",
		Password: "0213",
		Age:      25,
	}
	var users []any
	users = append(users, user1, user2)
	id, _, err := db.New().BatchInsert(users)
	if err != nil {
		panic(err)
	}
	fmt.Println(id)
}

func UpdateUser() {
	dataSourceName := fmt.Sprintf("root:0213@tcp(localhost:3306)/msgo?charset=utf8&loc=%s&parseTime=true", url.QueryEscape("Asia/Shanghai"))
	db := orm.Open("mysql", dataSourceName)
	db.Prefix = "t_"
	//user := &User{
	//	Age:      25,
	//	Username: "赵树下",
	//	Password: "111111111",
	//}
	//update, err := db.New().Where("id", 9).Update(user)
	update, err := db.New().Table("t_user").Where("id", 9).And().Where("age", 25).Update("age", 44, "username", "高野侯", "password", "22222222")
	if err != nil {
		panic(err)
	}
	fmt.Println(update)
}

func SelectOne() {
	dataSourceName := fmt.Sprintf("root:0213@tcp(localhost:3306)/msgo?charset=utf8&loc=%s&parseTime=true", url.QueryEscape("Asia/Shanghai"))
	db := orm.Open("mysql", dataSourceName)
	db.Prefix = "t_"
	user := &User{}
	err := db.New().Where("id", 9).SelectOne(user) //将查到的值赋给user
	if err != nil {
		panic(err)
	}
	fmt.Println(user)
}

func Delete() {
	dataSourceName := fmt.Sprintf("root:0213@tcp(localhost:3306)/msgo?charset=utf8&loc=%s&parseTime=true", url.QueryEscape("Asia/Shanghai"))
	db := orm.Open("mysql", dataSourceName)
	affect, err := db.New().Table("t_user").Where("age", 20).Delete()
	if err != nil {
		panic(err)
	}
	fmt.Println(affect)
}

func Select() {
	dataSourceName := fmt.Sprintf("root:0213@tcp(localhost:3306)/msgo?charset=utf8&loc=%s&parseTime=true", url.QueryEscape("Asia/Shanghai"))
	db := orm.Open("mysql", dataSourceName)
	db.Prefix = "t_"
	users, err := db.New().Where("age", 25).Select(&User{})
	if err != nil {
		panic(err)
	}
	fmt.Println(users)
	for _, v := range users {
		u := v.(*User)
		fmt.Println(*u)
	}
}
