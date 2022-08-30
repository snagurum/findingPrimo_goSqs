package main

import (
	"flag"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"

	"github.com/gin-gonic/gin"
)

var processPrimeQueueURL string = "https://sqs.us-east-1.amazonaws.com/898168850022/processPrimeQueue"
var processPrimeQueueName string = "processPrimeQueue"
var primeQueueURL string = "https://sqs.us-east-1.amazonaws.com/898168850022/primeQueue"
var primeQueueName string = "primeQueue"
var SLEEPTIME int64 = 4

var isProcessPrimeSlicePolling = false
var processPrimesSlice []*sqs.Message = make([]*sqs.Message, 10)
var isPrimeSlicePolling = false
var primesSlice []*sqs.Message = make([]*sqs.Message, 10)

//listQueues
//sendMessage
//getMessage
//deleteMessage
//primeQueue
//processPrimeQueue

func main() {

	opType := flag.String("opType", "", "operation type")
	queueName1 := flag.String("n", "processPrimeQueue", "queue name")
	flag.Parse()
	fmt.Printf("opType = %s, queueName = %s\n", *opType, *queueName1)

	if *opType == "listQueues" {
		fmt.Printf("LIST QUEUES \n ====================\n")
		ListQueues()
	} else if *opType == "sendMessage" {
		fmt.Printf("SEND MESSAGE \n ====================\n")
		SendMessage(&processPrimeQueueURL, 25)
	} else if *opType == "getMessage" {
		fmt.Printf("GET MESSAGE \n ====================\n")
		a, _ := GetMessage(&processPrimeQueueURL, processPrimeQueueName, 10, 1)
		fmt.Printf("receive message.... => %v\n", *a)
	} else if *opType == "deleteMessage" {
		fmt.Printf("DELETE MESSAGE \n ====================\n")
		a, _ := GetMessage(&processPrimeQueueURL, processPrimeQueueName, 10, 1)
		if nil != a.Messages {
			fmt.Printf("deleting message.... => %v\n", *a.Messages[0])
			DeleteMessage(&processPrimeQueueURL, a, nil)
		}
	} else if *opType == "primeQueue" {
		go processPrimeQueue()
		setupGin(":8082")
	} else if *opType == "processPrimeQueue" {
		setupGin(":8081")
	}
}

func getSvc() *sqs.SQS {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := sqs.New(sess)
	return svc
}

//=====================================================
func ListQueues() {
	svc := getSvc()
	result, err := svc.ListQueues(nil)
	if err != nil {
		fmt.Printf("error occured....%v\n", err)
		return
	}

	fmt.Println("printing sqs list \n=================")
	for i, url := range result.QueueUrls {
		fmt.Printf("%d: %s\n", i, *url)
	}
}

func SendMessage(queueURL *string, primeNumber int) error {
	svc := getSvc()

	_, err := svc.SendMessage(&sqs.SendMessageInput{
		DelaySeconds: aws.Int64(10),
		MessageBody:  aws.String(strconv.Itoa(primeNumber)),
		QueueUrl:     queueURL,
	})

	if err != nil {
		return err
	}

	return nil
}

func GetMessage(queueURL *string, queueName string, visibilitySec int, messageCount int) (*sqs.ReceiveMessageOutput, error) {
	svc := getSvc()

	var oneMin int64 = int64(visibilitySec)
	var oneMinPtr *int64 = &oneMin
	urlResult, err := svc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &queueName,
	})
	if err != nil {
		fmt.Printf("error1 %v", err)
	}
	var messageCount64 = int64(messageCount)
	msgResult, err := svc.ReceiveMessage(&sqs.ReceiveMessageInput{
		QueueUrl:            urlResult.QueueUrl,
		MaxNumberOfMessages: aws.Int64(messageCount64),
		VisibilityTimeout:   oneMinPtr,
		AttributeNames: aws.StringSlice([]string{
			"SentTimestamp",
		}),
		MessageAttributeNames: aws.StringSlice([]string{
			"All",
		}),
		WaitTimeSeconds: aws.Int64(10),
	})
	if err != nil {
		fmt.Printf("error2 %v", err)
		return nil, err
	}

	return msgResult, nil
}

func DeleteMessage(queueURL *string, messages *sqs.ReceiveMessageOutput, message *sqs.Message) {
	svc := getSvc()
	tempMessageReceiptHandle := ""
	if messages != nil {
		tempMessageReceiptHandle = *messages.Messages[0].ReceiptHandle
	} else if message != nil {
		tempMessageReceiptHandle = *message.ReceiptHandle
	}
	if tempMessageReceiptHandle != "" {
		_, err := svc.DeleteMessage(&sqs.DeleteMessageInput{
			QueueUrl:      queueURL,
			ReceiptHandle: &tempMessageReceiptHandle,
		})
		if err != nil {
			fmt.Printf("error2 %v", err)
			return
		}
	} else {
		fmt.Printf("Queue is empty %v", queueURL)
	}
}

func purgeSupplement(queueURL *string, queueName string) {
	fmt.Printf("purging queue %v \n", *queueURL)
	for {
		a, _ := GetMessage(queueURL, queueName, 10, 10)
		if a != nil && a.Messages != nil {
			for _, msg := range a.Messages {
				DeleteMessage(queueURL, nil, msg)
			}
		} else {
			break
		}
	}
	fmt.Printf("purging queue %v complete-----\n", *queueURL)
}

func purge(c *gin.Context) {
	go purgeSupplement(&processPrimeQueueURL, processPrimeQueueName)
	go purgeSupplement(&primeQueueURL, primeQueueName)
	c.String(200, "")
}

//=====================================================

func setupGin(port string) {
	router := gin.Default()

	router.GET("/ppq/evaluatePrime", evaluatePrime)
	router.GET("/ppq/getProcessPrimeQueue", getProcessPrimeQueue)
	router.GET("/pq/getPrimeQueue", getPrimeQueue)
	router.GET("/ppq/purge", purge)
	router.Run(port)
}

//=====================================================
func evaluatePrime(c *gin.Context) {
	number := c.DefaultQuery("number", "0")
	primeNum, _ := strconv.Atoi(number)
	SendMessage(&processPrimeQueueURL, primeNum)
	c.String(200, "%v", number)
}

func getProcessPrimeQueue(c *gin.Context) {
	if !isProcessPrimeSlicePolling {
		isProcessPrimeSlicePolling = true
		processPrimesSlice = make([]*sqs.Message, 10)
		a, _ := GetMessage(&processPrimeQueueURL, processPrimeQueueName, 10, 10)
		processPrimesSlice = append(processPrimesSlice, a.Messages...)
		isProcessPrimeSlicePolling = false
	}
	c.JSON(200, processPrimesSlice)
}

//=====================================================
func getPrimeQueue(c *gin.Context) {
	if !isPrimeSlicePolling {
		isPrimeSlicePolling = true
		primesSlice = make([]*sqs.Message, 10)
		a, _ := GetMessage(&primeQueueURL, primeQueueName, 10, 10)
		primesSlice = append(primesSlice, a.Messages...)
		isPrimeSlicePolling = false
	}
	c.JSON(200, primesSlice)
}

func isPrime(a int32) bool {
	for i := int32(2); i < a; i++ {
		if a%i == 0 {
			return false
		}
	}
	return true
}

func processPrimeQueue() {
	fmt.Print("process Prime Queue")
	for {
		a, _ := GetMessage(&processPrimeQueueURL, processPrimeQueueName, 10, 1)
		fmt.Printf(" retrieved message %v\n", a)
		if nil != a.Messages {
			tempNumber, _ := strconv.Atoi(*a.Messages[0].Body)
			if isPrime(int32(tempNumber)) {
				SendMessage(&primeQueueURL, tempNumber)
			}
			fmt.Printf("deleting message.... => %v\n", *a.Messages[0])
			DeleteMessage(&processPrimeQueueURL, a, nil)
		}
		time.Sleep(time.Duration(SLEEPTIME) * time.Second)
	}
}
