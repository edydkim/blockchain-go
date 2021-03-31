package main

import (
	_ "bufio"
	_ "bytes"
	"challenge-go/cipher"
	"challenge-go/constants"
	"challenge-go/model"
	"challenge-go/utils"
	"encoding/csv"
	"fmt"
	"github.com/omise/omise-go"
	"github.com/omise/omise-go/operations"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	csvRecord   = make(model.Record)
	donation    model.Donation
	atomCounter = AtomCounter{v: make(map[string]int64)}
)

type AtomCounter struct {
	mu sync.Mutex
	v  map[string]int64
	lc int
}

func (a *AtomCounter) inc(key string, amount int64) {
	a.mu.Lock()
	a.v[key] += amount
	a.lc++
	a.mu.Unlock()
}

func (a *AtomCounter) value(key string) (int64, int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.v[key], a.lc
}

type CSVError struct {
	when time.Time
	what string
}

func (e *CSVError) Error() string {
	return fmt.Sprintf("at %v, %s", e.when, e.what)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

/**
 * Convert to CSV
 */
func toCSV(s string) {
	csvReader := csv.NewReader(strings.NewReader(s))

	cr, _ := csvReader.Read()

	if len(cr) < 6 {
		log.Println(&CSVError{time.Now(), "Wrong CSV Record..."})
	}

	amount, _ := strconv.ParseInt(cr[1], 10, 64)
	expMonth, _ := strconv.Atoi(cr[4])
	expYear, _ := strconv.Atoi(cr[5])
	csvRecord.Add(cr[0], model.Item{Amount: amount, Ccn: cr[2], Cvv: cr[3], ExpMonth: expMonth, ExpYear: expYear})

	donation = model.Donation{Records: &csvRecord}
}

/**
 * Call creating-token API
 */
func token(client *omise.Client, name string, ccn string, expMonth int, expYear int, ch chan string) {
	// Creates a token
	card, createToken := &omise.Card{}, &operations.CreateToken{
		Name:            name,
		Number:          ccn,
		ExpirationMonth: time.Month(expMonth),
		ExpirationYear:  expYear,
	}

	client.Do(card, createToken)

	ch <- card.ID
}

/**
 * Call charge API with Card
 */
func charge(client *omise.Client, amount int64, card string) {
	if len(card) == 0 {
		atomCounter.inc("fail", amount)
		return
	}

	// Creates a charge from the token
	charge, create := &omise.Charge{}, &operations.CreateCharge{
		Amount:   amount,
		Currency: "thb", // Is it valid currency to call the charge API?!
		Card:     card,
	}

	if e := client.Do(charge, create); e != nil {
		atomCounter.inc("fail", amount)
	}

	atomCounter.inc("success", amount)
}

/**
 * Call creating-token and charge API in thread
 */
func callAPI(client *omise.Client) {
	fmt.Println("performing donations...")

	ch := make(chan string)

	top3 := [3]int64{0}
	pred, suc := "", ""
	cur, next := int64(0), int64(0)
	for key, ele := range *donation.Records {
		for cnt := range ele {
			cur = ele[cnt].Amount
			pred = key
			for i := 0; i < 3; i++ {
				if cur > top3[i] {
					suc = donation.Top3[i]

					next = top3[i]
					top3[i] = cur
					cur = next

					donation.Top3[i] = pred
					pred = suc
				}
			}

			go token(client, key, ele[cnt].Ccn, ele[cnt].ExpMonth, ele[cnt].ExpYear, ch)

			go charge(client, ele[cnt].Amount, <-ch)
		}
	}

	fmt.Println("performing donations...Done")
}

/**
 * Summarize output
 */
func summary() {
	successLc, failLc := 0, 0
	donation.Success, successLc = atomCounter.value("success")
	donation.Failed, failLc = atomCounter.value("fail")
	donation.Total = donation.Success + donation.Failed
	donation.Average = donation.Total / int64(successLc+failLc)

	fmt.Printf("       total received: %15v\n", donation.Total)
	fmt.Printf(" successfully donated: %15v\n", donation.Success)
	fmt.Printf("      faulty donation: %15v\n", donation.Failed)
	fmt.Printf("   average per person: %15v\n", donation.Average)
	fmt.Printf("           top donors: \n")
	for i := range donation.Top3 {
		fmt.Printf("                   %15v\n", donation.Top3[i])
	}
}

/**
 * Main
 */
func main() {
	csvFilename := os.Args[1]
	f, err := os.Open(csvFilename)
	if err != nil {
		log.Fatalf("Cannot open file %q: %v", csvFilename, err)
	}
	defer f.Close()

	c, err := cipher.NewRot128Reader(f)
	check(err)

	// Init API Client
	resourceManager := utils.ResourceManager{}
	client, e := omise.NewClient(resourceManager.GetProperty(constants.OMISE_PUBLIC_KEY),
		resourceManager.GetProperty(constants.OMISE_SECRET_KEY))
	if e != nil {
		log.Fatal(e)
	}

	seeker := 0
	buf := make([]byte, 1024*4)
	for {
		_, err := c.Read(buf)

		newLine := 0
		for n := range buf {
			if buf[n] == '\n' {
				newLine = n
				seeker += newLine + 1
				f.Seek(int64(seeker), io.SeekStart)
				break
			}
		}

		// Skip header
		if seeker == newLine+1 {
			continue
		}

		toCSV(string(buf[:newLine]))

		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
	}

	callAPI(client)

	summary()
}
