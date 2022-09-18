package gateway

import "strings"

//前缀树
type TreeNode struct {
	Name     string
	Children []*TreeNode
	GwName   string //节点响应路径，不包含路由组的名称
	IsEnd    bool   //判断是否是尾节点
}

func (t *TreeNode) Put(path string, gwName string) { //添加路由节点
	strs := strings.Split(path, "/")
	for index, name := range strs {
		if index == 0 { //忽略第一个空格
			continue
		}
		children := t.Children
		isMatch := false
		for _, node := range children {
			if node.Name == name {
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
			node := &TreeNode{Name: name, Children: make([]*TreeNode, 0), IsEnd: isEnd, GwName: gwName}
			children = append(children, node)
			t.Children = children
			t = node
		}
	}
}

func (t *TreeNode) Get(path string) *TreeNode { //返回最后一个匹配的node，未匹配到返回nil
	strs := strings.Split(path, "/")
	for index, name := range strs {
		if index == 0 { //忽略第一个空格
			continue
		}
		children := t.Children
		isMatch := false
		for _, node := range children {
			if node.Name == name || node.Name == "*" || strings.Contains(node.Name, ":") {
				isMatch = true
				t = node
				if index == len(strs)-1 {
					return node
				}
				break
			}
		}
		if !isMatch {
			for _, node := range children {
				if node.Name == "**" { //用于路由尾部, 如/order/get未匹配到，会被 /order/**匹配到（如果有）
					return node
				}
			}
		}
	}
	return nil
}
