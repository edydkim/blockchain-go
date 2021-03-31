package model

type Item struct {
	Amount   int64
	Ccn      string
	Cvv      string
	ExpMonth int
	ExpYear  int
}

type Record map[string][]Item

type Donation struct {
	Total, Success, Failed int64
	Average                int64
	Top3                   [3]string
	Records                *Record
}

func (r Record) Add(name string, i Item) *Record {
	r[name] = append(r[name], i)
	return &r
}

func (d Donation) Add(r *Record) {
	d.Records = r
}
