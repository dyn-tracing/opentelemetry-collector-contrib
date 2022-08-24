package main

import (
    "encoding/json"
    "os"
    "reflect"
    "fmt"
)

// type Reference struct {
//     refType string
// 	traceID string
// 	spanID  string
// }

// type Tag struct {
//     key   string
// 	type  string
// 	value string
// }

// type Field struct {
//     key   string
// 	type  string
// 	value string //multiple types
// }

// type Log struct {
//     timestamp int
// 	Fields    []Field
// }

// //undetermined
// type process struct {
//     serviceName string
//     Tags        []Tag
// }

// type Span struct {
//     traceID       string
//     spanID        string
//     operationName string
//     References    []Reference
//     startTime     int
//     duration      int
//     Tags          []Tag
//     Logs          []Log
//     processID     string
//     warnings      string //type unknown
// }

// type Processes struct {
//     //incomplete
// }

// type datum struct {
//     traceID   string
//     Spans     []Span
//     Processes Processes //depends on previous changes
//     warnings  string
// }

// type EntireJson struct {
// 	data   []datum
//     total  int
//     limit  int
//     offset int
//     error  string //unknown
// }

type Service struct {
	spanID      string
	parentID    string
	processType string
}

func extractSpans(fileName string) ([]Service, [][]int) {
    content, err := os.ReadFile(fileName)
    if err != nil {
        panic(err)
    }

    serviceList := make([]Service, 0)

    var all map[string]interface{}
    err = json.Unmarshal([]byte(content), &all)
    if err != nil {
        panic(err)
    }

    dataList, ok := all["data"].([]interface{})
    if !ok {
        panic("dataList is not a list!")
    }

    for i := 0; i < len(dataList); i++ {
        dataMap, ok1 := dataList[i].(map[string]interface{})
        if !ok1 {
            panic("dataMap is not a map!")
        }
        spanList, ok2 := dataMap["spans"].([]interface{})
        if !ok2 {
            panic("spanList is not a list!")
        }
        processMap, ok3 := dataMap["processes"].(map[string]interface{})
        if !ok3 {
            panic("processMap is not a map!")
        }

        for j := 0; j < len(spanList); j++ {
            spanMap, ok1 := spanList[j].(map[string]interface{})
            if !ok1 {
                panic("spanMap is not a map!")
            }
            spanID, ok2 := spanMap["spanID"]
            if !ok2 {
                panic("spanMap[spanID] is not a string!")
            }
            var parentID string
            referenceList, ok3 := spanMap["references"].([]interface{})
            if !ok3 {
                panic("referenceList is not a list!")
            }
            if len(referenceList) > 0 {
                referenceMap, ok4 := referenceList[0].(map[string]interface{})
                if !ok4 {
                    panic("referenceMap is not a map!")
                }
                parentID = referenceMap["spanID"].(string)
            }

            processTypeMap, ok7 := processMap[spanMap["processID"].(string)].(map[string]interface{})
            if !ok7 {
                panic("processTypeMap is not a map!")
            }
            processType := processTypeMap["serviceName"].(string)

            service := Service{
                spanID: spanID.(string),
	            parentID: parentID,
	            processType: processType,
            }
            serviceList = append(serviceList, service)
        }
    }

    serviceChildList := make([][]int, len(serviceList))
    
    for i := range serviceList {
        parent := serviceList[i].parentID
        if ( parent != ""){
            for j := range serviceList {
                if(serviceList[j].spanID == parent){
                    serviceChildList[j] = append(serviceChildList[j],i)
                    break
                }
            }
        }
    }

    return serviceList, serviceChildList
}

func main() {
    serviceList1, serviceChildList1 := extractSpans("test1.json")
    serviceList2, serviceChildList2 := extractSpans("test2.json")
    serviceList3, serviceChildList3 := extractSpans("test3.json")

    //test 1
    sampleServiceList1 := []Service {
        Service {
            spanID: "0",
            parentID: "",
            processType: "processName1",
        },
        Service {
            spanID: "1",
            parentID: "0",
            processType: "processName2",
        },
        Service {
            spanID: "2",
            parentID: "1",
            processType: "processName3",
        },
        Service {
            spanID: "3",
            parentID: "0",
            processType: "processName4",
        },
    }

    sampleServiceChildList1 := [][]int {{1,3},{2},{},{}}

    serviceOk1 := true
    for i := range serviceList1 {
         if !reflect.DeepEqual(serviceList1[i], sampleServiceList1[i]) {
            serviceOk1 = false
            break;
         }
    }

    serviceChildOk1 := true
    for i := range serviceChildList1 {
        for j := range serviceChildList1[i] {
            if serviceChildList1[i][j] != sampleServiceChildList1[i][j] {
                serviceChildOk1 = false
                break;
            }
        }
    }

    if !serviceOk1 {
        panic("serviceList for test1 is incorrect!")
    }
    if !serviceChildOk1 {
        panic("serviceChildList for test1 is incorrect!")
    }

    fmt.Println("test1 passed!")

    //printing results to compare
    // fmt.Println("The result is: ")
    // for i := range serviceList1 {
    //     // res, _ := json.MarshalIndent(serviceList1[i], "", "\t")
    //     // fmt.Println(string(res), " ")
    //     fmt.Println("item ", i, ":")
    //     fmt.Println("spanID: ", serviceList1[i].spanID)
    //     fmt.Println("parentID: ", serviceList1[i].parentID)
    //     fmt.Println("processName: ", serviceList1[i].processType)
    // }
    // fmt.Println("")
    // fmt.Println("The sample result is: ")
    // for j := range sampleServiceList1 {
    //     // res, _ := json.MarshalIndent(sampleServiceList1[j], "", "\t")
    //     // fmt.Println(string(res), " ")
    //     fmt.Println("item ", j, ":")
    //     fmt.Println("spanID: ", sampleServiceList1[j].spanID)
    //     fmt.Println("parentID: ", sampleServiceList1[j].parentID)
    //     fmt.Println("processName: ", sampleServiceList1[j].processType)
    // }

    //test 2
    sampleServiceList2 := []Service {
        Service {
            spanID: "0",
            parentID: "",
            processType: "processName1",
        },
        Service {
            spanID: "1",
            parentID: "0",
            processType: "processName2",
        },
        Service {
            spanID: "2",
            parentID: "0",
            processType: "processName3",
        },
        Service {
            spanID: "3",
            parentID: "0",
            processType: "processName4",
        },
    }

    sampleServiceChildList2 := [][]int {{1,2,3},{},{},{}}

    serviceOk2 := true
    for i := range serviceList2 {
         if !reflect.DeepEqual(serviceList2[i], sampleServiceList2[i]) {
            serviceOk2 = false
            break;
         }
    }

    serviceChildOk2 := true
    for i := range serviceChildList2 {
        for j := range serviceChildList2[i] {
            if serviceChildList2[i][j] != sampleServiceChildList2[i][j] {
                serviceChildOk2 = false
                break;
            }
        }
    }

    if !serviceOk2 {
        panic("serviceList for test2 is incorrect!")
    }
    if !serviceChildOk2 {
        panic("serviceChildList for test2 is incorrect!")
    }

    fmt.Println("test2 passed!")

    //test 3
    sampleServiceList3 := []Service {
        Service {
            spanID: "0",
            parentID: "",
            processType: "processName1",
        },
        Service {
            spanID: "1",
            parentID: "0",
            processType: "processName2",
        },
        Service {
            spanID: "2",
            parentID: "1",
            processType: "processName3",
        },
        Service {
            spanID: "3",
            parentID: "2",
            processType: "processName4",
        },
    }

    sampleServiceChildList3 := [][]int {{1},{2},{3},{}}

    serviceOk3 := true
    for i := range serviceList3 {
         if !reflect.DeepEqual(serviceList3[i], sampleServiceList3[i]) {
            serviceOk3 = false
            break;
         }
    }

    serviceChildOk3 := true
    for i := range serviceChildList3 {
        for j := range serviceChildList3[i] {
            if serviceChildList3[i][j] != sampleServiceChildList3[i][j] {
                serviceChildOk3 = false
                break;
            }
        }
    }

    if !serviceOk3 {
        panic("serviceList for test3 is incorrect!")
    }
    if !serviceChildOk3 {
        panic("serviceChildList for test3 is incorrect!")
    }

    fmt.Println("test3 passed!")
}

