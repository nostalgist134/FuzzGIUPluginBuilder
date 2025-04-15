package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

/*
	-t 模板类型
	-o 输出路径
	-path 文件路径
	-gopath 使用的golang路径
	-g 指定目录，在目录下生成一个环境包含fuzztype库和特定类型的plugin
	builder -t plgen/reactor/plproc/preproc -path pluginFile.go -o xxx.dll
	builder -t xxx -g C:/path/
	-> go -buildmode=c-shared wrappedPlugin.go -o xxx.dll
	参数
	-t 必须
	-o, -path -o非必须，未指定则使用pluginFunName，-path如果不使用-g则必须，如果使用-g，这两项被忽略
	-g, -gopath 非必须
*/

func main() {
	templateType := flag.String("t", "", "template type, can be "+
		"payloadProc,reactor,payloadGen,reqSender or preprocess")
	pluginPath := flag.String("build", "", "plugin file or directory to build the plugin. "+
		"if the path is directory, file \"plugin.go\" in the directory will be used")
	outputFileName := flag.String("o", "", "output file name")
	goPath := flag.String("gopath", "", "go binary path be used to build the plugin")
	genPath := flag.String("gen", "", "path to generate go project for plugin"+
		"(collocate with -t)")
	keepIntermidiate := flag.Bool("keep-intermediate", false, "keep intermediate files")
	flag.Parse()
	//
	if *genPath == "" && *pluginPath == "" { // 编译插件和生成开发目录必须至少一个
		fmt.Println("plugin path or generate path is required")
		os.Exit(1)
	}
	if *templateType == "" {
		fmt.Println("template type is required")
		os.Exit(1)
	}
	if *goPath == "" { // 未指明golang路径，直接执行go命令
		*goPath = "go"
	}
	goVersion := exec.Command(*goPath, "version")
	goVer, err := goVersion.Output()
	if err != nil { // 执行失败，说明golang环境无效
		panic(err)
	}
	goVer = bytes.Split(goVer, []byte(" "))[2][2:] // 获取golang版本
	fmt.Printf("Using go version - %s\n", goVer)
	var pluginFunName string
	// 根据不同类型的插件，查找不同的函数名
	switch *templateType {
	case "payloadProc":
		pluginFunName = "PayloadProcessor"
	case "reactor":
		pluginFunName = "React"
	case "payloadGen":
		pluginFunName = "PayloadGenerator"
	case "preprocess":
		pluginFunName = "Preprocessor"
	case "reqSender":
		pluginFunName = "ReqSender"
	default:
		fmt.Printf("Unsupported template type: %s\n", *templateType)
		os.Exit(1)
	}
	/*
		在指定目录下生成插件开发环境：
			1.创建plugin.go, go.mod, 将fuzzTypes.go复制到components/fuzzTypes/fuzzTypes.go
			2.根据templateType从gotmp模板中选择一个，把函数取出来填写到plugin.go中
	*/
	if *genPath != "" {
		gen(genPath, templateType, pluginFunName, goVer)
		return
	}
	fmt.Println("Plugin type: " + pluginFunName)
	if *outputFileName == "" {
		*outputFileName = "FuzzGIU" + pluginFunName + ".dll"
		fmt.Printf("Output file name %s\n", *outputFileName)
	}
	isFile, err := IsFile(*pluginPath)
	if err != nil {
		panic(err)
	}

	if !isFile { // 如果路径是目录，则采用目录下的plugin.go文件
		*pluginPath += "/plugin.go"
	}
	// 获取插件文件的相关信息
	pluginImports, err := getImports(*pluginPath)
	if err != nil {
		panic(err)
	}
	params, err := getFuncParams(*pluginPath, pluginFunName) // 解析插件文件的函数参数列表
	if err != nil {
		panic(err)
	}
	completeParamList := params
	retType, err := getFuncReturnType(*pluginPath, pluginFunName) // 解析返回类型
	if err != nil {
		panic(err)
	}
	code, err := GetCodes(*pluginPath) // 获取文件中的源码部分（import之后开始的部分）
	if err != nil {
		panic(err)
	}
	// 判断插件函数参数列表的函数签名是否符合定义，并删去params列表中的固定参数
	switch *templateType {
	case "payloadProc":
		if params[0].Type != "string" || params[0].Name != "payload" ||
			retType != "string" {
			fmt.Println("bad function definition, example: " +
				"PayloadProcessor(payload string, {custom arguments}) string")
			os.Exit(1)
		}
		params = params[1:]
	case "reactor":
		if params[0].Type != "*fuzzTypes.Req" || params[0].Name != "request" ||
			params[1].Type != "*fuzzTypes.Resp" || params[1].Name != "resp" ||
			retType != "*fuzzTypes.Reaction" {
			fmt.Println("bad function definition, example: " +
				"React(request *fuzzTypes.Req, resp *fuzzTypes.Resp, {custom arguments}) *fuzzTypes.Reaction")
			os.Exit(1)
		}
		params = params[2:]
	case "preprocess":
		if params[0].Type != "*fuzzTypes.Fuzz" || params[0].Name != "fuzz" ||
			retType != "*fuzzTypes.Fuzz" {
			fmt.Println("bad function definition, example: " +
				"Preprocessor(fuzz fuzzTypes.Fuzz, {custom arguments}) *fuzzTypes.Fuzz")
			os.Exit(1)
		}
		params = params[1:]
	case "payloadGen":
		if retType != "[]string" {
			fmt.Println("bad function definition, example: " +
				"PayloadGenerator({custom arguments}) []string")
			os.Exit(1)
		}
	case "reqSender":
		if params[0].Type != "*fuzzTypes.SendMeta" || params[0].Name != "sendMeta" ||
			retType != "*fuzzTypes.Resp" {
			fmt.Println("bad function definition, example: " +
				"ReqSender(sendMeta *fuzzTypes.SendMeta, {custom arguments}) *fuzzTypes.Resp")
			os.Exit(1)
		}
	}

	formalParamsStr := ""
	actualParamsStr := ""
	for i, param := range params { // 拼接参数字符串
		formalParamsStr += fmt.Sprintf("%s %s", param.Name, param.Type)
		actualParamsStr += param.Name
		if i != len(params)-1 {
			formalParamsStr += ", "
			actualParamsStr += ", "
		}
	}
	// 模板文件名，格式为tmpl+首字母大写的插件类型+.gotmp，在当前目录的templates子目录下
	tmplFileName := "templates/" + "tmpl" +
		strings.ToUpper((*templateType)[:1]) + (*templateType)[1:] + ".gotmp"
	tmplImports, err := getImports(tmplFileName)
	// 去除插件go文件中与模板文件重合的import
	for i, pImport := range pluginImports {
		for _, tImport := range tmplImports {
			if pImport == tImport {
				pluginImports[i] = ""
				break
			}
		}
	}
	dedupImports := "import (\n"
	// custom import语句
	for _, pImport := range pluginImports {
		dedupImports += fmt.Sprintf("\t%s\n", pImport)
	}
	dedupImports += ")"
	if err != nil {
		panic(err)
	}
	tmpl, err := os.ReadFile(tmplFileName)
	if err != nil {
		panic(err)
	}
	// 将模板文件与插件文件合并
	tmpl = bytes.Replace(tmpl, []byte("/* FORMAL PARAMETERS */"), []byte(formalParamsStr), -1)
	tmpl = bytes.Replace(tmpl, []byte("/* ACTUAL PARAMETERS */"), []byte(actualParamsStr), -1)
	tmpl = bytes.Replace(tmpl, []byte("/* CODE */"), []byte(code), -1)
	tmpl = bytes.Replace(tmpl, []byte("/* CUSTOM IMPORTS */"), []byte(dedupImports), -1)
	dir, _ := GetFileDir(*pluginPath)
	err = os.Chdir(dir) // 切换到插件所在目录
	if err != nil {
		panic(err)
	}
	wrappedPlugin, err := os.Create("wrappedPlugin.go") // 编译过程中生成的临时文件
	if err != nil {
		panic(err)
	}
	_, err = wrappedPlugin.Write(tmpl)
	if err != nil {
		panic(err)
	}
	wrappedPlugin.Close()
	if !*keepIntermidiate {
		defer os.Remove("wrappedPlugin.go")                                // 临时文件编译结束后删除
		defer os.Remove(strings.Replace(*outputFileName, ".dll", ".h", 1)) // 删除编译时生成的.h文件
	}
	build := exec.Command(*goPath, "build", "-buildmode=c-shared", "-ldflags=-s", "-ldflags=-w", "-o",
		*outputFileName, "./wrappedPlugin.go")
	output, err := build.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		panic(err)
	}
	fmt.Printf("Successfully built %s, plugin type - %s\n", *outputFileName, *templateType)
	fmt.Printf("Plugin parameters - %v", completeParamList)
	return
}
