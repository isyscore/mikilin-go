package mikilin

import (
	"fmt"
	matcher "github.com/SimonAlong/Mikilin-go/match"
	log "github.com/sirupsen/logrus"
	"reflect"
	"strconv"
	"strings"
)

type Matcher interface {
	Match(object interface{}, field reflect.StructField, fieldValue interface{}) bool
	IsEmpty() bool
	GetWhitMsg() string
	GetBlackMsg() string
}

type FieldMatcher struct {

	// 属性名
	fieldName string
	// 异常名字
	errMsg string
	// 是否接受：true，则表示白名单，false，则表示黑名单
	accept bool
	// 是否禁用
	disable bool
	// 待转换的名字
	changeTo string
	// 匹配器列表
	Matchers []Matcher
}

type InfoCollector func(objectTypeName string, fieldKind reflect.Kind, objectFieldName string, subCondition string)

type CollectorEntity struct {
	name         string
	infCollector InfoCollector
}

type CheckResult struct {
	Result bool
	ErrMsg string
}

var checkerEntities []CollectorEntity

/* key：类全名，value：key：属性名 */
var matcherMap = make(map[string]map[string]FieldMatcher)

func Check(object interface{}, fieldNames ...string) (bool, string) {
	objType := reflect.TypeOf(object)
	objValue := reflect.ValueOf(object)

	if objValue.Kind() == reflect.Ptr && !objValue.IsNil() {
		objValue = objValue.Elem()
	}

	if objValue.Kind() != reflect.Struct {
		return true, ""
	}

	fmt.Println(objType.String())
	ch := make(chan *CheckResult)
	for index, num := 0, objType.NumField(); index < num; index++ {
		field := objType.Field(index)

		if !inArray(field.Name, fieldNames...) {
			continue
		}

		//fieldKind := objValue.Field(index).Kind()
		// 非核查类型则返回
		//if !isCheckedBaseKing(fieldKind) {
		//	continue
		//} else if fieldKind == reflect.Struct {
		//
		//}

		tagJudge := field.Tag.Get(MATCH)
		if len(tagJudge) == 0 {
			continue
		}

		// 搜集核查器
		if _, contain := matcherMap[objType.String()][field.Name]; !contain {
			collectChecker(objType.String(), objValue.Field(index).Kind(), field.Name, tagJudge)
		}

		// 核查结果：任何一个属性失败，则返回失败
		go check(object, field, objValue.Field(index).Interface(), ch)
		checkResult := <-ch
		if !checkResult.Result {
			close(ch)
			return false, checkResult.ErrMsg
		}
	}
	close(ch)
	return true, ""
}

func inArray(fieldName string, fieldNames ...string) bool {
	for _, name := range fieldNames {
		name = strings.ToUpper(name[:1]) + name[1:]
		if name == fieldName {
			return true
		}
	}
	return false
}

func collectChecker(objectName string, fieldKind reflect.Kind, fieldName string, matchJudge string) {
	subCondition := strings.Split(matchJudge, ";")
	for _, subStr := range subCondition {
		subStr = strings.TrimSpace(subStr)
		buildChecker(objectName, fieldKind, fieldName, subStr)
	}
}

func buildChecker(objectName string, fieldKind reflect.Kind, fieldName string, subStr string) {
	for _, entity := range checkerEntities {
		entity.infCollector(objectName, fieldKind, fieldName, subStr)
	}
}

func check(object interface{}, field reflect.StructField, fieldValue interface{}, ch chan *CheckResult) {
	objectType := reflect.TypeOf(object)
	if fieldMatcher, contain := matcherMap[objectType.String()][field.Name]; contain {
		accept := fieldMatcher.accept
		matchers := fieldMatcher.Matchers
		for _, match := range matchers {
			if match.IsEmpty() {
				continue
			}

			matchResult := match.Match(object, field, fieldValue)
			if accept {
				if !matchResult {
					// 白名单，没有匹配上则返回false
					ch <- &CheckResult{Result: false, ErrMsg: match.GetWhitMsg()}
					return
				}
			} else {
				if matchResult {
					// 黑名单，匹配上则返回false
					ch <- &CheckResult{Result: false, ErrMsg: match.GetBlackMsg()}
					return
				}
			}
		}
	}
	ch <- &CheckResult{Result: true}
	return
}

// 包的初始回调
func init() {
	/* 搜集匹配后的操作参数 */
	//checkerEntities = append(checkerEntities, CollectorEntity{ERR_MSG, collectErrMsg})
	//checkerEntities = append(checkerEntities, CollectorEntity{CHANGE_TO, collectChangeTo})
	//checkerEntities = append(checkerEntities, CollectorEntity{ACCEPT, collectAccept})
	//checkerEntities = append(checkerEntities, CollectorEntity{DISABLE, collectDisable})

	/* 搜集匹配器 */
	checkerEntities = append(checkerEntities, CollectorEntity{VALUE, buildValuesMatcher})
	//checkerEntities = append(checkerEntities, CollectorEntity{IS_NIL, buildIsNilMatcher})
	//checkerEntities = append(checkerEntities, CollectorEntity{IS_BLANK, buildIsBlankMatcher})
	//checkerEntities = append(checkerEntities, CollectorEntity{RANGE, buildRangeMatcher})
	//checkerEntities = append(checkerEntities, CollectorEntity{MODEL, buildModelMatcher})
	//checkerEntities = append(checkerEntities, CollectorEntity{ENUM_TYPE, buildEnumTypeMatcher})
	//checkerEntities = append(checkerEntities, CollectorEntity{CONDITION, buildConditionMatcher})
	//checkerEntities = append(checkerEntities, CollectorEntity{CUSTOMIZE, buildCustomizeMatcher})
	//checkerEntities = append(checkerEntities, CollectorEntity{REGEX, buildRegexMatcher})
}

func collectErrMsg(objectTypeName string, objectFieldName string, subCondition string) {

}

func collectChangeTo(objectTypeName string, objectFieldName string, subCondition string) {

}

func collectAccept(objectTypeName string, objectFieldName string, subCondition string) {

}

func collectDisable(objectTypeName string, objectFieldName string, subCondition string) {

}

func buildValuesMatcher(objectTypeName string, fieldKind reflect.Kind, objectFieldName string, subCondition string) {
	if !strings.Contains(subCondition, VALUE) || !strings.Contains(subCondition, EQUAL) {
		return
	}

	index := strings.Index(subCondition, "=")
	value := subCondition[index+1:]

	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		value = value[1 : len(value)-1]
		var availableValues []interface{}
		for _, subValue := range strings.Split(value, ",") {
			subValue = strings.TrimSpace(subValue)
			if chgValue, err := cast(fieldKind, subValue); err == nil {
				availableValues = append(availableValues, chgValue)
			} else {
				log.Error(err.Error())
			}
		}
		valueMatch := matcher.ValueMatch{Values: availableValues}

		var matchers []Matcher
		matchers = append(matchers, &valueMatch)

		// 添加匹配器到map
		fieldMatcher, contain := matcherMap[objectTypeName][objectFieldName]
		if !contain {
			matcherMap[objectTypeName] = make(map[string]FieldMatcher)
			fieldMatcher = FieldMatcher{fieldName: objectFieldName, Matchers: matchers, accept: true}
		} else {
			fieldMatcher.Matchers = append(fieldMatcher.Matchers, matchers...)
		}
		matcherMap[objectTypeName][objectFieldName] = fieldMatcher
	}
}

func buildIsNilMatcher(objectTypeName string, objectFieldName string, subCondition string) {

}

func buildIsBlankMatcher(objectTypeName string, objectFieldName string, subCondition string) {

}

func buildRangeMatcher(objectTypeName string, objectFieldName string, subCondition string) {

}

func buildModelMatcher(objectTypeName string, objectFieldName string, subCondition string) {

}

func buildEnumTypeMatcher(objectTypeName string, objectFieldName string, subCondition string) {

}

func buildConditionMatcher(objectTypeName string, objectFieldName string, subCondition string) {

}

func buildRegexMatcher(objectTypeName string, objectFieldName string, subCondition string) {

}

func buildCustomizeMatcher(objectTypeName string, objectFieldName string, subCondition string) {

}

// 判断是否是核查的基本类型
func isCheckedBaseKing(fieldKing reflect.Kind) bool {
	switch fieldKing {
	case reflect.Int:
		return true
	case reflect.Int8:
		return true
	case reflect.Int16:
		return true
	case reflect.Int32:
		return true
	case reflect.Int64:
		return true
	case reflect.Uint:
		return true
	case reflect.Uint8:
		return true
	case reflect.Uint16:
		return true
	case reflect.Uint32:
		return true
	case reflect.Uint64:
		return true
	case reflect.Float32:
		return true
	case reflect.Float64:
		return true
	case reflect.Bool:
		return true
	default:
		return false
	}
}

func isMapKing(fieldKind reflect.Kind) {
	if fieldKind == reflect.Struct {

	}
}

func cast(fieldKind reflect.Kind, valueStr string) (interface{}, error) {
	switch fieldKind {
	case reflect.Int:
		return strconv.Atoi(valueStr)
	case reflect.Int8:
		return strconv.ParseInt(valueStr, 10, 8)
	case reflect.Int16:
		return strconv.ParseInt(valueStr, 10, 16)
	case reflect.Int32:
		return strconv.ParseInt(valueStr, 10, 32)
	case reflect.Int64:
		return strconv.ParseInt(valueStr, 10, 64)
	case reflect.Uint:
		return strconv.ParseUint(valueStr, 10, 0)
	case reflect.Uint8:
		return strconv.ParseUint(valueStr, 10, 8)
	case reflect.Uint16:
		return strconv.ParseUint(valueStr, 10, 16)
	case reflect.Uint32:
		return strconv.ParseUint(valueStr, 10, 32)
	case reflect.Uint64:
		return strconv.ParseUint(valueStr, 10, 64)
	case reflect.Float32:
		return strconv.ParseFloat(valueStr, 32)
	case reflect.Float64:
		return strconv.ParseFloat(valueStr, 64)
	case reflect.Bool:
		return strconv.ParseBool(valueStr)
	}

	return valueStr, nil
}
