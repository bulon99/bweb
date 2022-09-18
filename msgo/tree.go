package msgo

import "strings"

//前缀树
type treeNode struct {
	name       string
	children   []*treeNode
	routerName string //节点响应路径，不包含路由组的名称
	isEnd      bool   //判断是否是尾节点
}

func (t *treeNode) Put(path string) { //添加路由节点
	strs := strings.Split(path, "/")
	for index, name := range strs {
		if index == 0 { //忽略第一个空格
			continue
		}
		children := t.children
		isMatch := false
		for _, node := range children {
			if node.name == name {
				isMatch = true
				t = node
				break
			}
		}
		if !isMatch { //未匹配到生成新node
			isEnd := false
			if index == len(strs)-1 {
				isEnd = true
			}
			node := &treeNode{name: name, children: make([]*treeNode, 0), isEnd: isEnd}
			children = append(children, node)
			t.children = children
			t = node
		}
	}
}

func (t *treeNode) Get(path string) *treeNode { //返回最后一个匹配的node，未匹配到返回nil
	strs := strings.Split(path, "/")
	routerName := ""
	for index, name := range strs {
		if index == 0 { //忽略第一个空格
			continue
		}
		children := t.children
		isMatch := false
		for _, node := range children {
			if node.name == name || node.name == "*" || strings.Contains(node.name, ":") {
				isMatch = true
				routerName += "/" + node.name
				node.routerName = routerName
				t = node
				if index == len(strs)-1 {
					return node
				}
				break
			}
		}
		if !isMatch {
			for _, node := range children {
				if node.name == "**" { //**用于路由尾部
					routerName += "/" + node.name
					node.routerName = routerName
					return node
				}
			}
		}
	}
	return nil
}
