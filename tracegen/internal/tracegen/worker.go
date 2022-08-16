// Copyright The OpenTelemetry Authors
// Copyright (c) 2018 The Jaeger Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tracegen // import "github.com/open-telemetry/opentelemetry-collector-contrib/tracegen/internal/tracegen"

import (
        "context"
        "sync"
        "sync/atomic"
        "time"
        "math/rand"
        "go.opentelemetry.io/otel"
        "go.opentelemetry.io/otel/attribute"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
        semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
        "go.opentelemetry.io/otel/trace"
        "go.uber.org/zap"
        "golang.org/x/time/rate"
)

type worker struct {
        running          *uint32         // pointer to shared flag that indicates it's time to stop the test
        numTraces        int             // how many traces the worker has to generate (only when duration==0)
        propagateContext bool            // whether the worker needs to propagate the trace context via HTTP headers
        totalDuration    time.Duration   // how long to run the test for (overrides `numTraces`)
        limitPerSecond   rate.Limit      // how many spans per second to generate
        wg               *sync.WaitGroup // notify when done
        logger           *zap.Logger
        traceTypes        int
        serviceNames      [12]string
    tracerProviders  []*sdktrace.TracerProvider
}

const (
        fakeIP string = "1.2.3.4"

        fakeSpanDuration = 100000 * time.Microsecond
)

func (w worker) setUpTracers() []trace.Tracer {
    toReturn := make([]trace.Tracer, 0, len(w.tracerProviders))

    for i := 0; i< len(w.tracerProviders); i++ {
        otel.SetTracerProvider(w.tracerProviders[i])
        tracer := otel.Tracer("tracegen"+string(i))
        toReturn = append(toReturn, tracer)
    }
    return toReturn
}

func (w worker) addChild(parentCtx context.Context, tracer trace.Tracer, message string, serviceName string, httpStatusCode string, httpUrl string) context.Context {
    childCtx, child := tracer.Start(parentCtx, message, trace.WithAttributes(
        attribute.String("span.kind", getRandSpanKind()), // is there a semantic convention for this?
        attribute.String("service.name", serviceName),
        semconv.HTTPStatusCodeKey.String(httpStatusCode),
        semconv.HTTPURLKey.String(httpUrl),
    ))
    opt := trace.WithTimestamp(time.Now().Add(fakeSpanDuration))
    child.End(opt)
    return childCtx
}

// input a range and get a random number within that range
func getRandomNum(min int, max int) int{
    rand.Seed(time.Now().UnixNano())
    return rand.Intn(max - min+1) + min
}


// get a status code based on hardcoded probability
func getRandStatusCode() string {
    var statusCode string
    randNum := getRandomNum(1, 10)
    switch randNum {
        case 1, 2, 3, 4, 5: statusCode = "200"
        case 6: statusCode = "500"
        case 7: statusCode = "202"
        case 8: statusCode = "402"
        case 9: statusCode = "300"
        case 10: statusCode = "404"
        // just in case of error
        default: statusCode = "Error"
    }
    return statusCode
}

func getRandSpanKind() string {
    var spanKind string
    randNum := getRandomNum(1, 2)
    switch randNum {
        case 1: spanKind = "client"
        case 2: spanKind = "server"
        // just in case of error
        default: spanKind = "N/A"
    }
    return spanKind
}

// need exception handling!
func (w worker) getRootAttribute(servicesIndex int) (string, string, string, string){
    //one status code and url for entire tree
    spanKind := getRandSpanKind()
    serviceName := w.serviceNames[servicesIndex]
    httpStatusCode := getRandStatusCode()
    httpUrl := "http://metadata.google.internal/computeMetadata/v1/instance/attributes/cluster-name(fakeurl)"
    return spanKind, serviceName, httpStatusCode, httpUrl
}


func (w worker) simulateTraces() {
    //Read the file named sampleTrace.json in the same directory and extract the tree of services
    //The serviceList contains all services with its spanID, parentID, serviceType, and processType.
    //The serviceChildList contains the lists of children services corresponding to the index in serviceList.
    content, err := os.ReadFile("sampleTrace.json")
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
            //todo: get servicetype
            var serviceType string
            tagList, ok5 := spanMap["tags"].([]interface{})
            if !ok5 {
                panic("tagList is not a list!")
            }
            for k := 0; k < len(tagList); k++ {
                tagMap, ok6 := tagList[k].(map[string]interface{})
                if !ok6 {
                    panic("tagMap is not a map!")
                }
                if tagMap["key"] == "rpc.service" {
                    serviceType = tagMap["value"].(string)
                    break
                }
            }
            processTypeMap, ok7 := processMap[spanMap["processID"].(string)].(map[string]interface{})
            if !ok7 {
                panic("processTypeMap is not a map!")
            }
            processType := processTypeMap["serviceName"].(string)

            service := Service{
                spanID: spanID.(string),
	            parentID: parentID,
	            serviceType: serviceType,
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
    
    // set up all tracers
    tracers := w.setUpTracers()
    limiter := rate.NewLimiter(w.limitPerSecond, 1)
    var i int
    for atomic.LoadUint32(w.running) == 1 {
        t := i%w.traceTypes
        if t == 0 {
            w.simulateTrace1(limiter, tracers)
        } else if t == 1 {
            w.simulateTrace2(limiter, tracers)
        } else if t == 2 {
            w.simulateTrace3(limiter, tracers)
        } else if t == 3 {
            w.simulateTrace4(limiter, tracers)
        } else if t == 4 {
            w.simulateTrace5(limiter, tracers)
        }
        i++
        if w.numTraces != 0 {
                if i >= w.numTraces {
                        break
                }
        }
    }
    w.logger.Info("traces generated", zap.Int("traces", i))
    w.wg.Done()
}


func (w worker) simulateTrace1(limiter *rate.Limiter, tracers []trace.Tracer) {
    spanKind, serviceName, httpStatusCode, httpUrl := w.getRootAttribute(0)
    ctx, sp := tracers[0].Start(context.Background(), "lets-go", trace.WithAttributes(
        attribute.String("span.kind", spanKind), // is there a semantic convention for this?
        attribute.String("service.name", serviceName),
        semconv.HTTPStatusCodeKey.String(httpStatusCode),
        semconv.HTTPURLKey.String(httpUrl),
    ))

    child1Ctx := w.addChild(ctx, tracers[0], "1", w.serviceNames[0], httpStatusCode, httpUrl)
    w.addChild(child1Ctx, tracers[4], "1-1", w.serviceNames[4], httpStatusCode, httpUrl)

    w.addChild(ctx, tracers[0], "2", w.serviceNames[0], httpStatusCode, httpUrl)

    child3Ctx := w.addChild(ctx, tracers[0], "3", w.serviceNames[3], httpStatusCode, httpUrl)
    grandChild3Ctx := w.addChild(child3Ctx, tracers[8], "3-1", w.serviceNames[8], httpStatusCode, httpUrl)
    greatGrandChild3Ctx := w.addChild(grandChild3Ctx, tracers[8], "3-1-1", w.serviceNames[8], httpStatusCode, httpUrl)
    w.addChild(greatGrandChild3Ctx, tracers[7], "3-1-1-1", w.serviceNames[7], httpStatusCode, httpUrl)

    for i := 0; i< 5; i++ {
        child4Ctx := w.addChild(ctx, tracers[0], "4", w.serviceNames[0], httpStatusCode, httpUrl)
        w.addChild(child4Ctx, tracers[7], "4-1", w.serviceNames[7], httpStatusCode, httpUrl)
    }

    child5Ctx := w.addChild(ctx, tracers[0], "5", w.serviceNames[0], httpStatusCode, httpUrl)
    w.addChild(child5Ctx, tracers[10], "5-1", w.serviceNames[10], httpStatusCode, httpUrl)

    for i := 0; i< 2; i++ {
        child6Ctx := w.addChild(ctx, tracers[0], "6", w.serviceNames[0], httpStatusCode, httpUrl)
        w.addChild(child6Ctx, tracers[4], "6-1", w.serviceNames[4], httpStatusCode, httpUrl)
    }

    limiter.Wait(context.Background())
    opt := trace.WithTimestamp(time.Now().Add(fakeSpanDuration))
    sp.End(opt)
}

func (w worker) simulateTrace2(limiter *rate.Limiter, tracers []trace.Tracer) {
   spanKind, serviceName, httpStatusCode, httpUrl := w.getRootAttribute(0)
   ctx, sp := tracers[0].Start(context.Background(), "lets-go", trace.WithAttributes(
        attribute.String("span.kind", spanKind), // is there a semantic convention for this?
        attribute.String("service.name", serviceName),
        semconv.HTTPStatusCodeKey.String(httpStatusCode),
        semconv.HTTPURLKey.String(httpUrl),
    ))


    child1Ctx := w.addChild(ctx, tracers[0], "1", w.serviceNames[0],httpStatusCode, httpUrl)
    w.addChild(child1Ctx, tracers[4], "1-1", w.serviceNames[4], httpStatusCode, httpUrl)

    child2Ctx := w.addChild(ctx, tracers[0], "2", w.serviceNames[0], httpStatusCode, httpUrl)
    w.addChild(child2Ctx, tracers[7], "2-1", w.serviceNames[7], httpStatusCode, httpUrl)

    w.addChild(ctx, tracers[0], "3", w.serviceNames[0], httpStatusCode, httpUrl)

    for i := 0; i< 9; i++ {
        child4Ctx := w.addChild(ctx, tracers[0], "4", w.serviceNames[0], httpStatusCode, httpUrl)
        w.addChild(child4Ctx, tracers[4], "4-1", w.serviceNames[4], httpStatusCode, httpUrl)
    }

    child5Ctx := w.addChild(ctx, tracers[0], "5", w.serviceNames[0], httpStatusCode, httpUrl)
    w.addChild(child5Ctx, tracers[1], "5-1", w.serviceNames[1], httpStatusCode, httpUrl)

    limiter.Wait(context.Background())

    opt := trace.WithTimestamp(time.Now().Add(fakeSpanDuration))
    sp.End(opt)
 
}

func (w worker) simulateTrace3(limiter *rate.Limiter, tracers []trace.Tracer) {
    spanKind, serviceName, httpStatusCode, httpUrl := w.getRootAttribute(0)
    ctx, sp := tracers[0].Start(context.Background(), "lets-go", trace.WithAttributes(
        attribute.String("span.kind", spanKind), // is there a semantic convention for this?
        attribute.String("service.name", serviceName),
        semconv.HTTPStatusCodeKey.String(httpStatusCode),
        semconv.HTTPURLKey.String(httpUrl),
    ))
    
    //1st inheritance
    childCtx1 := w.addChild(ctx, tracers[0], "1", w.serviceNames[0], httpStatusCode, httpUrl)
    w.addChild(childCtx1, tracers[7], "1-1", w.serviceNames[7], httpStatusCode, httpUrl)

    //2nd inheritance
    w.addChild(ctx, tracers[0], "2", w.serviceNames[0], httpStatusCode, httpUrl)

    limiter.Wait(context.Background())
    opt := trace.WithTimestamp(time.Now().Add(fakeSpanDuration))
    sp.End(opt)
}

func (w worker) simulateTrace4(limiter *rate.Limiter, tracers []trace.Tracer) {
    spanKind, serviceName, httpStatusCode, httpUrl := w.getRootAttribute(0)
    ctx, sp := tracers[0].Start(context.Background(), "lets-go", trace.WithAttributes(
        attribute.String("span.kind", spanKind), // is there a semantic convention for this?
        attribute.String("service.name", serviceName),
        semconv.HTTPStatusCodeKey.String(httpStatusCode),
        semconv.HTTPURLKey.String(httpUrl),
    ))
    //number starts from 1 instead of 0
    //1st inheritance
    childCtx1 := w.addChild(ctx, tracers[0], "1", w.serviceNames[0], httpStatusCode, httpUrl)
    grandChildCtx1_1 := w.addChild(childCtx1, tracers[3], "1-1", w.serviceNames[3], httpStatusCode, httpUrl)

    w.addChild(grandChildCtx1_1, tracers[3], "1-1-1", w.serviceNames[3], httpStatusCode, httpUrl)

    greatGrandChildCtx1_1_2 := w.addChild(grandChildCtx1_1, tracers[3], "1-1-2", w.serviceNames[3], httpStatusCode, httpUrl)
    w.addChild(greatGrandChildCtx1_1_2, tracers[7], "1-1-2-1", w.serviceNames[7], httpStatusCode, httpUrl)

    greatGrandChildCtx1_1_3 := w.addChild(grandChildCtx1_1, tracers[3], "1-1-3", w.serviceNames[3], httpStatusCode, httpUrl)
    w.addChild(greatGrandChildCtx1_1_3, tracers[10], "1-1-3-1", w.serviceNames[10], httpStatusCode, httpUrl)

    greatGrandChildCtx1_1_4 := w.addChild(grandChildCtx1_1, tracers[3], "1-1-4", w.serviceNames[3], httpStatusCode, httpUrl)
    w.addChild(greatGrandChildCtx1_1_4, tracers[6], "1-1-4-1", w.serviceNames[6], httpStatusCode, httpUrl)

    greatGrandChildCtx1_1_5 := w.addChild(grandChildCtx1_1, tracers[3], "1-1-5", w.serviceNames[3], httpStatusCode, httpUrl)
    w.addChild(greatGrandChildCtx1_1_5, tracers[10], "1-1-5-1", w.serviceNames[10], httpStatusCode, httpUrl)

    w.addChild(grandChildCtx1_1, tracers[3], "1-1-6", w.serviceNames[3], httpStatusCode, httpUrl)
    
    greatGrandChildCtx1_1_7 := w.addChild(grandChildCtx1_1, tracers[3], "1-1-7", w.serviceNames[3], httpStatusCode, httpUrl)
    w.addChild(greatGrandChildCtx1_1_7, tracers[5], "1-1-7-1", w.serviceNames[5], httpStatusCode, httpUrl)

    //2nd inheritance
    childCtx2 := w.addChild(ctx, tracers[0], "2", w.serviceNames[0], httpStatusCode, httpUrl)
    grandChildCtx2_1 := w.addChild(childCtx2, tracers[8], "2-1", w.serviceNames[8], httpStatusCode, httpUrl)
    greatGrandChildCtx2_1_1 := w.addChild(grandChildCtx2_1, tracers[8], "2-1-1", w.serviceNames[8], httpStatusCode, httpUrl)
    w.addChild(greatGrandChildCtx2_1_1, tracers[7], "2-1-1-1", w.serviceNames[7], httpStatusCode, httpUrl)

    //3rd inheritance
    for i := 0; i<5; i++ {
        childCtx3 := w.addChild(ctx, tracers[0], "3", w.serviceNames[0], httpStatusCode, httpUrl)
        w.addChild(childCtx3, tracers[7], "3-1", w.serviceNames[7], httpStatusCode, httpUrl)
    }

    //4th inheritance
    childCtx4 := w.addChild(ctx, tracers[0], "4", w.serviceNames[0], httpStatusCode, httpUrl)
    w.addChild(childCtx4, tracers[4], "4-1", w.serviceNames[4], httpStatusCode, httpUrl)

    limiter.Wait(context.Background())
    opt := trace.WithTimestamp(time.Now().Add(fakeSpanDuration))
    sp.End(opt)
}

func (w worker) simulateTrace5(limiter *rate.Limiter, tracers []trace.Tracer) {
    spanKind, serviceName, httpStatusCode, httpUrl := w.getRootAttribute(0)
    ctx, sp := tracers[0].Start(context.Background(), "lets-go", trace.WithAttributes(
        attribute.String("span.kind", spanKind), // is there a semantic convention for this?
        attribute.String("service.name", serviceName),
        semconv.HTTPStatusCodeKey.String(httpStatusCode),
        semconv.HTTPURLKey.String(httpUrl),
    ))
    
    //1st inheritance
    for i := 0; i<6; i++ {
        childCtx1 := w.addChild(ctx, tracers[0], "1", w.serviceNames[0], httpStatusCode, httpUrl)
        w.addChild(childCtx1, tracers[7], "1-1", w.serviceNames[7], httpStatusCode, httpUrl)
    }

    //2nd inheritance
    childCtx2 := w.addChild(ctx, tracers[0], "2", w.serviceNames[0], httpStatusCode, httpUrl)
    w.addChild(childCtx2, tracers[4], "2-1", w.serviceNames[4], httpStatusCode, httpUrl)

    //3rd inheritance
    w.addChild(ctx, tracers[0], "3", w.serviceNames[0], httpStatusCode, httpUrl)

    //4th inheritance
    childCtx4 := w.addChild(ctx, tracers[0], "4", w.serviceNames[0], httpStatusCode, httpUrl)
    w.addChild(childCtx4, tracers[4], "4-1", w.serviceNames[4], httpStatusCode, httpUrl)

    //5th inheritance
    childCtx5 := w.addChild(ctx, tracers[0], "5", w.serviceNames[0], httpStatusCode, httpUrl)
    grandChildCtx5_1 := w.addChild(childCtx5, tracers[8], "5-1", w.serviceNames[8], httpStatusCode, httpUrl)
    greatGrandChildCtx5_1_1 := w.addChild(grandChildCtx5_1, tracers[8], "5-1-1", w.serviceNames[8], httpStatusCode, httpUrl)
    w.addChild(greatGrandChildCtx5_1_1, tracers[7], "5-1-1-1", w.serviceNames[7], httpStatusCode, httpUrl)

    //6th inheritance
    childCtx6 := w.addChild(ctx, tracers[0], "6", w.serviceNames[0], httpStatusCode, httpUrl)
    w.addChild(childCtx6, tracers[1], "6-1", w.serviceNames[1], httpStatusCode, httpUrl)

    limiter.Wait(context.Background())
    opt := trace.WithTimestamp(time.Now().Add(fakeSpanDuration))
    sp.End(opt)
}