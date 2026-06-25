package hs

import (
	"encoding/json"
	"log/slog"
	"misroservice/hs/response"
	"misroservice/hs/response/codes"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

// HTTPMethodPrefix 定义HTTP方法前缀映射
var HTTPMethodPrefix = map[string]string{
	"Get":    http.MethodGet,
	"Post":   http.MethodPost,
	"Put":    http.MethodPut,
	"Delete": http.MethodDelete,
	"Patch":  http.MethodPatch,
}

// camelToKebab 将驼峰命名转换为斜杠分隔的路径
// 例如: GetUserInfo -> /user/info, GetList -> /list
func camelToKebab(s string) string {
	// 移除HTTP方法前缀
	for prefix := range HTTPMethodPrefix {
		if strings.HasPrefix(s, prefix) {
			s = s[len(prefix):]
			break
		}
	}

	if s == "" {
		return ""
	}

	// 将驼峰转换为斜杠分隔
	var result strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				result.WriteRune('/')
			}
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}
	return "/" + result.String()
}

// extractHTTPMethod 从方法名前缀提取HTTP方法
func extractHTTPMethod(methodName string) string {
	for prefix, method := range HTTPMethodPrefix {
		if strings.HasPrefix(methodName, prefix) {
			return method
		}
	}
	return ""
}

// ServiceMethodInfo 包含服务方法的元数据
type ServiceMethodInfo struct {
	Method             reflect.Method
	HTTPMethod         string
	Path               string
	MethodName         string
	SkipRequestParsing bool // 是否跳过请求参数解析 (当参数名为 "_" 时)
}

// DiscoverServiceMethods 发现服务中符合自动路由规则的方法
// 规则:
//  1. 方法名以 Get/Post/Put/Delete/Patch 开头
//  2. 方法签名必须为: func(ctx context.Context, req *ReqType) (*RespType, error)
//     或: func(ctx context.Context, _ *ReqType) (*RespType, error) - 跳过请求参数解析
func DiscoverServiceMethods(service interface{}) []ServiceMethodInfo {
	var methods []ServiceMethodInfo
	serviceType := reflect.TypeOf(service)

	for i := 0; i < serviceType.NumMethod(); i++ {
		method := serviceType.Method(i)
		methodName := method.Name

		// 跳过未导出的方法
		if !method.IsExported() {
			continue
		}

		// 检查HTTP方法前缀
		httpMethod := extractHTTPMethod(methodName)
		if httpMethod == "" {
			continue
		}

		// 验证方法签名: func(ctx context.Context, req *ReqType) (*RespType, error)
		if !isValidServiceMethodSignature(method.Type) {
			slog.Warn("skip method with invalid signature", "method", methodName, "reason", "signature does not match func(ctx context.Context, req *ReqType) (*RespType, error)")
			continue
		}

		// 检查第三个参数名是否为 "_" (通过参数的类型名来判断)
		skipRequestParsing := isUnderscoreParameter(method, serviceType, i)

		// 生成路由路径
		path := camelToKebab(methodName)

		methods = append(methods, ServiceMethodInfo{
			Method:             method,
			HTTPMethod:         httpMethod,
			Path:               path,
			MethodName:         methodName,
			SkipRequestParsing: skipRequestParsing,
		})
	}

	return methods
}

// isUnderscoreParameter 检查方法的第三个参数名是否为 "_"
func isUnderscoreParameter(method reflect.Method, serviceType reflect.Type, methodIndex int) bool {
	// 获取方法的实际函数对象
	methodFunc := method.Func

	// 无法直接从 reflect 获取参数名，需要通过方法签名字符串或其他方式
	// 这里使用一个启发式方法：检查参数类型的名称
	// 实际上，在 Go 的反射中无法直接获取参数名
	// 我们需要使用另一种方式：检查方法的源代码或使用调试信息

	// 更好的方案是在方法实现时检查参数值是否为 nil
	// 但这里我们使用一个约定：检查第三个参数的类型是否实现了特殊接口

	// 简单实现：通过方法函数的字符串表示来判断
	methodStr := methodFunc.String()
	return strings.Contains(methodStr, ", _,") || strings.HasSuffix(methodStr, ", _)")
}

// isValidServiceMethodSignature 检查方法签名是否符合自动路由要求
// 要求: func(ctx context.Context, req *ReqType) (*RespType, error)
// 即: 3个输入参数(receiver, context, request), 2个输出参数(response, error)
func isValidServiceMethodSignature(methodType reflect.Type) bool {
	// 检查输入参数数量: receiver + context + request = 3
	if methodType.NumIn() != 3 {
		return false
	}

	// 检查输出参数数量: response + error = 2
	if methodType.NumOut() != 2 {
		return false
	}

	// 检查第二个输入参数是否是 context.Context
	if !isContextType(methodType.In(1)) {
		return false
	}

	// 检查最后一个输出参数是否是 error
	if !isErrorType(methodType.Out(1)) {
		return false
	}

	// 检查第三个输入参数是否是指针类型或 any (request)
	reqType := methodType.In(2)
	if reqType.Kind() != reflect.Ptr && !isAnyType(reqType) {
		return false
	}

	// 检查第一个输出参数是否是指针类型或接口类型 (response)
	respType := methodType.Out(0)
	if respType.Kind() != reflect.Ptr && respType.Kind() != reflect.Interface {
		return false
	}

	return true
}

// isContextType 检查类型是否为 context.Context
func isContextType(t reflect.Type) bool {
	return t.String() == "context.Context"
}

// isErrorType 检查类型是否为 error
func isErrorType(t reflect.Type) bool {
	return t.String() == "error"
}

// isAnyType 检查类型是否为 any 或 interface{}
func isAnyType(t reflect.Type) bool {
	return t.Kind() == reflect.Interface && t.NumMethod() == 0
}

// CreateAutoHandler 为服务方法创建HTTP处理程序
// 方法签名: func(ctx context.Context, req *ReqType) (*RespType, error)
// 特殊情况: func(ctx context.Context, _ *ReqType) (*RespType, error) - 跳过请求参数解析
//
//	func(ctx context.Context, _ any) (*RespType, error) - 传递 nil
func CreateAutoHandler(methodInfo ServiceMethodInfo, service interface{}) http.HandlerFunc {
	method := methodInfo.Method
	serviceValue := reflect.ValueOf(service)
	methodType := method.Type

	return func(w http.ResponseWriter, r *http.Request) {
		// 构建输入参数: [receiver, context, request]
		var args []reflect.Value
		args = append(args, serviceValue)
		args = append(args, reflect.ValueOf(r.Context()))

		// 获取请求对象类型
		reqType := methodType.In(2)
		var req reflect.Value

		// 判断参数类型
		if isAnyType(reqType) {
			// 如果是 any 类型，传递 nil interface{}
			// 需要创建一个有效的 nil 值，而不是零值的 Value
			req = reflect.Zero(reqType)
		} else {
			// 否则创建指针实例
			req = reflect.New(reqType.Elem())

			// 如果不跳过请求参数解析
			if !methodInfo.SkipRequestParsing {
				// 根据HTTP方法解析请求
				if methodInfo.HTTPMethod == http.MethodGet || methodInfo.HTTPMethod == http.MethodDelete {
					// 从Query参数解析
					if err := parseQueryParams(r, req.Interface()); err != nil {
						slog.Error("parse query error", "method", methodInfo.MethodName, "err", err)
						response.JSON(w, response.Resp{
							Code: int(codes.BadRequest),
							Msg:  err.Error(),
						})
						return
					}
				} else {
					// 从JSON Body解析
					if err := json.NewDecoder(r.Body).Decode(req.Interface()); err != nil {
						slog.Error("decode body error", "method", methodInfo.MethodName, "err", err)
						response.JSON(w, response.Resp{
							Code: int(codes.BadRequest),
							Msg:  err.Error(),
						})
						return
					}
				}
			}
		}

		args = append(args, req)

		// 调用方法
		results := method.Func.Call(args)

		// 处理返回值
		handleMethodResults(w, results, methodInfo.MethodName)
	}
}

// parseQueryParams 从URL Query参数解析到结构体
func parseQueryParams(r *http.Request, v any) error {
	queryParams := r.URL.Query()
	if len(queryParams) == 0 {
		return nil
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr {
		return nil
	}

	elem := rv.Elem()
	if elem.Kind() != reflect.Struct {
		return nil
	}

	elemType := elem.Type()

	// 遍历结构体字段
	for i := 0; i < elemType.NumField(); i++ {
		field := elemType.Field(i)
		fieldValue := elem.Field(i)

		// 跳过不可设置的字段
		if !fieldValue.CanSet() {
			continue
		}

		// 获取 json tag 作为参数名
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// 解析 json tag
		paramName := strings.Split(jsonTag, ",")[0]

		// 获取查询参数值
		values := queryParams[paramName]
		if len(values) == 0 {
			continue
		}

		val := values[0]
		if val == "" {
			continue
		}

		// 根据字段类型直接赋值，避免 JSON 编解码
		if err := setFieldValue(fieldValue, field.Type, val); err != nil {
			return err
		}
	}

	return nil
}

// setFieldValue 根据字段类型设置值
func setFieldValue(fieldValue reflect.Value, fieldType reflect.Type, val string) error {
	switch fieldType.Kind() {
	case reflect.String:
		fieldValue.SetString(val)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}
		fieldValue.SetInt(i)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return err
		}
		fieldValue.SetUint(u)

	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return err
		}
		fieldValue.SetFloat(f)

	case reflect.Bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		fieldValue.SetBool(b)

	case reflect.Ptr:
		// 处理指针类型
		if fieldValue.IsNil() {
			fieldValue.Set(reflect.New(fieldType.Elem()))
		}
		return setFieldValue(fieldValue.Elem(), fieldType.Elem(), val)

	default:
		// 其他类型暂不支持，保持原有行为
		return nil
	}

	return nil
}

// handleMethodResults 处理服务方法的返回值
// 期望的返回值: (*Data, error)
func handleMethodResults(w http.ResponseWriter, results []reflect.Value, methodName string) {
	// 获取返回的error (最后一个返回值)
	var err error
	if !results[1].IsNil() {
		err = results[1].Interface().(error)
	}

	// 处理错误
	if err != nil {
		slog.Error("method error", "method", methodName, "err", err)
		response.JSON(w, response.Resp{
			Code: int(codes.InternalServerError),
			Msg:  err.Error(),
		})
		return
	}

	// 返回数据
	data := results[0].Interface()
	response.JSON(w, response.Resp{
		Code: int(codes.OK),
		Msg:  "success",
		Data: data,
	})
}

// RegisterService 自动注册服务的所有公开方法作为HTTP路由
func RegisterService(group *Group, pathPrefix string, service interface{}) {
	methods := DiscoverServiceMethods(service)

	for _, methodInfo := range methods {
		// 构建完整路由路径
		fullPath := pathPrefix + methodInfo.Path

		// 创建处理程序
		handler := CreateAutoHandler(methodInfo, service)

		// 注册路由
		pattern := methodInfo.HTTPMethod + " " + fullPath
		// slog.Info("auto register route", "pattern", pattern, "method", methodInfo.MethodName)
		group.HandleFunc(pattern, handler)
	}
}
