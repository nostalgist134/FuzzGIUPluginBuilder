package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"strings"
)

// Param 参数结构体
type Param struct {
	Name string
	Type string
}

// 检查参数是否已经存在于切片中（去重）
func contains(params []Param, name, typ string) bool {
	for _, p := range params {
		if p.Name == name && p.Type == typ {
			return true
		}
	}
	return false
}

// 合并字符串
func joinStrings(strings []string) string {
	result := ""
	for i, s := range strings {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}

// 将 AST 表达式转换为字符串（获取参数类型）
func exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name // 基本类型（int, string, etc.）
	case *ast.ArrayType:
		return "[]" + exprToString(t.Elt) // 切片类型，如 []int
	case *ast.StarExpr:
		return "*" + exprToString(t.X) // 指针类型，如 *int
	case *ast.SelectorExpr:
		return exprToString(t.X) + "." + t.Sel.Name // 结构体或包名.类型
	case *ast.FuncType:
		return "func(...)"
	default:
		return fmt.Sprintf("%T", expr) // 其他未知类型
	}
}

// 解析 Go 文件，提取指定函数的参数列表（返回参数数组）
func getFuncParams(filename, funcName string) ([]Param, error) {
	// 创建 token 集合
	fset := token.NewFileSet()

	// 读取文件内容
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// 解析 Go 文件
	node, err := parser.ParseFile(fset, filename, src, parser.AllErrors)
	if err != nil {
		return nil, err
	}

	// 存储参数的切片
	var params []Param
	funcFound := false

	// 遍历 AST 以找到目标函数
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			if fn.Name.Name == funcName {
				funcFound = true
				// 遍历参数列表
				for _, param := range fn.Type.Params.List {
					paramType := exprToString(param.Type) // 获取类型信息
					for _, paramName := range param.Names {
						if !contains(params, paramName.Name, paramType) {
							params = append(params, Param{Name: paramName.Name, Type: paramType})
						}
					}
				}
			}
		}
		return true
	})
	if !funcFound {
		return nil, errors.New("function " + funcName + " not found")
	}
	return params, nil
}

// 获取函数返回类型
func getReturnType(fn *ast.FuncDecl) string {
	// 检查函数是否有返回值
	if fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		return "void" // 没有返回值
	}

	// 处理多个返回值的情况
	var resultTypes []string
	for _, result := range fn.Type.Results.List {
		resultTypes = append(resultTypes, exprToString(result.Type))
	}

	// 单个返回值
	if len(resultTypes) == 1 {
		return resultTypes[0]
	}
	// 如果有多个返回值，返回合并的类型
	return fmt.Sprintf("(%s)", joinStrings(resultTypes))
}

func getFuncReturnType(filename, funcName string) (string, error) {
	// 创建 token 集合
	fset := token.NewFileSet()

	// 读取文件内容
	src, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}

	// 解析 Go 文件
	node, err := parser.ParseFile(fset, filename, src, parser.AllErrors)
	if err != nil {
		return "", err
	}
	var returnType string
	// 遍历 AST 以找到目标函数
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			if fn.Name.Name == funcName {
				// 获取返回类型
				returnType = getReturnType(fn)
			}
		}
		return true
	})
	strings.HasSuffix(returnType, "(")
	return returnType, nil
}

// GetCodes 返回 Go 文件中源码部分（从 import 语句结束到文件结束的内容）。
// 如果没有 import 语句，则返回整个文件
func GetCodes(filename string) (string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ImportsOnly)
	if err != nil {
		fmt.Println("Error parsing file:", err)
		return "", err
	}

	// 打开文件并读取其内容
	fileContent, err := os.ReadFile(filename)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return "", err
	}

	// 如果没有 import 语句，返回整个文件内容
	if len(node.Imports) == 0 {
		return string(fileContent), nil
	}

	// 获取最后一个 import 语句的结束位置
	lastImport := node.Imports[len(node.Imports)-1]
	endPos := lastImport.End()

	// 返回从 import 结束到文件末尾的内容
	return string(fileContent[endPos+1:]), nil
}

// 查找整个函数
func extractFunction(filename, funcName string) (string, error) {
	// 创建 token 文件集
	fset := token.NewFileSet()

	// 解析文件
	file, err := parser.ParseFile(fset, filename, nil, parser.AllErrors)
	if err != nil {
		return "", err
	}

	// 遍历 AST，查找函数
	for _, decl := range file.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			if funcDecl.Name.Name == funcName {
				// 将 AST 代码转换为字符串
				var sb strings.Builder
				if err := printer.Fprint(&sb, fset, funcDecl); err != nil {
					return "", err
				}
				return sb.String(), nil
			}
		}
	}

	return "", fmt.Errorf("function %s not found in %s", funcName, filename)
}

// 解析 Go 文件并提取所有 `import` 语句
func getImports(filename string) ([]string, error) {
	// 创建 token 集合
	fset := token.NewFileSet()

	// 读取文件内容
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// 解析 Go 文件
	node, err := parser.ParseFile(fset, filename, src, parser.AllErrors)
	if err != nil {
		return nil, err
	}

	// 存储 import 语句
	var imports []string
	for _, imp := range node.Imports {
		imports = append(imports, imp.Path.Value) // 直接使用 imp.Path.Value，保留引号
	}

	return imports, nil
}
