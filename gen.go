package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func getPluginFun(template string) string {
	switch template {
	case "payloadProc":
		return "func PayloadProcessor(payload string, /* CUSTOM ARGUMENTS */) string {\n}"
	case "reactor":
		return "func React(request *fuzzTypes.Req, resp *fuzzTypes.Resp, /* CUSTOM ARGUMENTS */) *fuzzTypes.Reaction {\n}"
	case "preprocess":
		return "func Preprocessor(fuzz *fuzzTypes.Fuzz, /* CUSTOM ARGUMENTS */) *fuzzTypes.Fuzz {\n}"
	case "payloadGen":
		return "func PayloadGenerator(/* CUSTOM ARGUMENTS */) []string {\n}"
	case "reqSender":
		return "func ReqSender(sendMeta *fuzzTypes.SendMeta, /* CUSTOM ARGUMENTS */) *fuzzTypes.Resp {\n}"
	}
	return ""
}

func gen(genPath *string, templateType *string, pluginFunName string, goVer []byte) {
	if *templateType == "" {
		fmt.Println("No template provided to generate")
		os.Exit(1)
	}
	err := os.MkdirAll(*genPath, 0755)
	if err != nil && !os.IsExist(err) {
		fmt.Printf("Error creating dir: %s - %v\n", *genPath, err)
		os.Exit(1)
	}
	err = os.MkdirAll(*genPath+"/components/fuzzTypes/", 0755)
	if err != nil {
		fmt.Printf("Error creating dir: %s\n", *genPath)
	}
	// 复制fuzzTypes.go声明文件到components/fuzzTypes/目录下
	err = copyFileToDir("./fuzzTypes/fuzzTypes.go", *genPath+"/components/fuzzTypes/")
	if err != nil {
		fmt.Printf("Failed to copy fuzzTypes.go to %s - %v\n", *genPath+"/components/fuzzTypes/", err)
	}
	pGoTmp, err := os.ReadFile("templates/plugin.gotmp") // plugin.go模板文件
	if err != nil {
		panic(err)
	}
	pluginFun := getPluginFun(*templateType)
	err = os.Chdir(*genPath) // 进入生成的目录
	if err != nil {
		panic(err)
	}
	fmt.Print("Generating go.mod...")
	goMod, err := os.Create("go.mod") // 创建go.mod文件
	if err != nil {
		log.Fatal(err)
	}
	defer goMod.Close()
	modName := pluginFunName + "FuzzGIU"
	mod := "module " + modName + "\n"
	version := "go " + string(goVer) + "\n"
	_, err = goMod.Write([]byte(mod + version))
	if err != nil {
		fmt.Print("Failed to write to go.mod.")
		os.Remove("go.mod")
		panic(err)
	}
	fmt.Printf("Done, go version %s\n", string(goVer))
	fmt.Printf("Creating plugin.go...")
	pluginGo, err := os.Create("plugin.go") // 创建plugin.go文件
	if err != nil {
		log.Fatal(err)
	}
	defer pluginGo.Close()
	// 替换模板数据
	pGoTmp = bytes.Replace(pGoTmp, []byte("/* MODULE NAME */"), []byte(modName), -1)
	pGoTmp = bytes.Replace(pGoTmp, []byte("/* PLUGIN FUNCTION */"), []byte(pluginFun), -1)
	_, err = pluginGo.Write(pGoTmp)
	if err != nil {
		panic(err)
	}
	absPath, err := filepath.Abs(*genPath)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Done. Successfully created plugin project at %s\n", absPath)
}
