package binding

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
)

type jsonBinding struct {
	DisallowUnknownFields bool
	IsValidate            bool
}

func (jsonBinding) Name() string {
	return "json"
}

func (b jsonBinding) Bind(r *http.Request, obj any) error {
	body := r.Body
	if body == nil {
		return errors.New("invalid request")
	}
	decoder := json.NewDecoder(r.Body)
	if b.DisallowUnknownFields {
		decoder.DisallowUnknownFields() //参数中传入了结构体中没有的字段，decoder.Decode()时报错
	}
	if b.IsValidate {
		err := validateRequireParam(obj, decoder) //结构体中必须的字段没有传，报错
		if err != nil {
			return err
		}
	} else {
		err := decoder.Decode(obj)
		if err != nil {
			return err
		}
	}
	//若能执行到这说明json参数符合要求
	return validate(obj) //使用github上的检验模块，在结构体中加入validate标签
}

func validateRequireParam(data any, decoder *json.Decoder) error {
	if data == nil {
		return errors.New("data is nil")
	}
	//将传入的json转为map，和结构体中字段对比
	valueOf := reflect.ValueOf(data) //先判断是否是指针类型
	if valueOf.Kind() != reflect.Pointer {
		return errors.New("no ptr type")
	}
	t := valueOf.Elem().Interface()
	of := reflect.ValueOf(t)
	switch of.Kind() {
	case reflect.Struct:
		return checkParam(of, data, decoder)
	case reflect.Slice, reflect.Array: //若是切片或数组类型
		elem := of.Type().Elem()
		if elem.Kind() == reflect.Struct {
			return checkParamSlice(elem, data, decoder)
		}
	}
	return nil
}

func checkParam(of reflect.Value, data any, decoder *json.Decoder) error {
	mapData := make(map[string]interface{})
	_ = decoder.Decode(&mapData)
	for i := 0; i < of.NumField(); i++ {
		field := of.Type().Field(i)
		tag := field.Tag.Get("json")
		required := field.Tag.Get("msgo")
		value := mapData[tag] //map获取不到struct中的某个字段，说明规定的字段json参数中没有，报错
		if value == nil && required == "required" {
			return errors.New(fmt.Sprintf("filed [%s] is required", tag))
		}
	}
	marshal, _ := json.Marshal(mapData)
	_ = json.Unmarshal(marshal, data)
	return nil
}

func checkParamSlice(elem reflect.Type, data any, decoder *json.Decoder) error {
	mapData := make([]map[string]interface{}, 0)
	_ = decoder.Decode(&mapData)
	if len(mapData) <= 0 {
		return nil
	}
	for _, mp := range mapData { //对其中的每一个都要校验
		for i := 0; i < elem.NumField(); i++ {
			field := elem.Field(i)
			required := field.Tag.Get("msgo")
			tag := field.Tag.Get("json")
			v := mp[tag]
			if v == nil && required == "required" {
				return errors.New(fmt.Sprintf("filed [%s] is required", tag))
			}
		}
	}
	marshal, _ := json.Marshal(mapData)
	_ = json.Unmarshal(marshal, data)
	return nil
}
