package common

import (
	"encoding/json"
	"fmt"
	"testing"
)

type User struct {
	Id  string
	Age int
}

func TestMap(t *testing.T) {
	m := make(map[string]*User)
	m["1"] = &User{Id: "1", Age: 1}
	for _, v := range m {
		v.Age = 10
	}
	marshal, _ := json.Marshal(m)
	fmt.Println(string(marshal))
}
func TestSlice(t *testing.T) {
	a := make([]int, 2, 2)
	a[0] = 1
	a[1] = 2
	fmt.Printf("%p \n", a)
	a = append(a, 100, 200, 300, 400)
	fmt.Printf("%p \n", a)
}
