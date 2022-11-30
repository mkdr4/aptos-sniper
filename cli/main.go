package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gookit/color"
	term "github.com/nsf/termbox-go"
	"github.com/paulrademacher/climenu"
	"golang.org/x/crypto/sha3"
)

// constant cli
const Version string = "1.0.0"
const Command = "clear"

type config_struct struct {
	Node    string `json:"aptos_node_url"`
	Key     string `json:"aptos_private_key"`
	Discord struct {
		Send_fail bool   `json:"send_fail"`
		Hook      string `json:"hook"`
	} `json:"discord_hook"`
	Collection struct {
		Topaz    collection_info_struct `json:"topaz"`
		Bluemove collection_info_struct `json:"bluemove"`
	} `json:"last_run_collection"`
	wallet struct {
		balance         string
		privateKey      ed25519.PrivateKey
		publicKey       ed25519.PublicKey
		address         [32]byte
		privateKeyStr   string
		publicKeyStr    string
		address_str     string
		sequence_number int
	}
	client struct {
		url          string //https://fullnode.mainnet.aptoslabs.com/v1
		accounts     string //https://fullnode.mainnet.aptoslabs.com/v1/accounts/
		encode       string //https://fullnode.mainnet.aptoslabs.com/v1/transaction/encode_submission
		transactions string //https://fullnode.mainnet.aptoslabs.com/v1/transactions
		result       string //https://fullnode.mainnet.aptoslabs.com/v1/transactions/by_hash/<<HASH>>
	}
}

type payload_struct struct {
	Type          string      `json:"type"`
	Function      string      `json:"function"`
	TypeArguments []string    `json:"type_arguments"`
	Arguments     interface{} `json:"arguments"`
}

type collection_info_struct struct {
	Name    string `json:"name"`
	ID      string `json:"id"`
	Creator string `json:"creator"`
}

type nft_info struct {
	token_name string
	price      float64
	rank       int
	image      string
	mode       string
}

type topaz_listing_struct struct {
	Error      bool   `json:"error"`
	Status     int    `json:"status"`
	StatusText string `json:"statusText"`
	Data       []struct {
		TokenID      string  `json:"token_id"`
		CollectionID string  `json:"collection_id"`
		TokenName    string  `json:"token_name"`
		IsListed     bool    `json:"is_listed"`
		Seller       string  `json:"seller"`
		Price        float64 `json:"price"`
		UpdatedAT    string  `json:"updated_at"`
		PreviewURI   string  `json:"preview_uri"`
		Rank         string  `json:"string,rank"`
	} `json:"data"`
}

type bluemove_listing_struct struct {
	Data []struct {
		ID         int `json:"id"`
		Attributes struct {
			Price      float64 `json:"price,string"`
			Name       string  `json:"name"`
			UpdatedAt  string  `json:"updatedAt"`
			URIMedia   string  `json:"uri_media"`
			Rank       int     `json:"rank"`
			Rarity     string  `json:"rarity"`
			Attributes []struct {
				Value     string `json:"value"`
				TraitType string `json:"trait_type"`
			} `json:"attributes"`
		} `json:"attributes"`
	} `json:"data"`
	Meta struct {
		Pagination struct {
			PageSize int `json:"pageSize"`
			Total    int `json:"total"`
		} `json:"pagination"`
	} `json:"meta"`
}

type bluemove_collections_struct struct {
	Data []struct {
		ID         int `json:"id"`
		Attributes struct {
			Name       string `json:"name"`
			Slug       string `json:"slug"`
			Creator    string `json:"creator"`
			UpdatedAt  string `json:"updatedAt"`
			FloorPrice string `json:"floor_price"`
		} `json:"attributes"`
	} `json:"data"`
	Meta struct {
		Pagination struct {
			Page      int `json:"page"`
			PageSize  int `json:"pageSize"`
			PageCount int `json:"pageCount"`
			Total     int `json:"total"`
		} `json:"pagination"`
	} `json:"meta"`
}

func main() {
	Clear(4, nil, nil)

	var Config config_struct
	if err := Config.load_config(); err != nil {
		color.Warn.Tips(fmt.Sprintf("%s", err))
		os.Exit(0)
	}

	logo(Config.wallet.balance)

	color.Grayf("Use arrows \u2191 \u2193 to navigate, space to select and enter to confirm and esc to back")

	for {
		fmt.Println()

		menu := climenu.NewButtonMenu("", "Choose an action")
		menu.AddMenuItem("Aptos sniper", "aptos_sniper")
		menu.AddMenuItem("Settings", "settings")

		action, escaped := menu.Run()
		if escaped {
			Clear(25, nil, nil)
			return
		}

		switch action {
		case "aptos_sniper":
			aptos_sniper(&Config)
			Clear(4, nil, nil)
		case "settings":
			settings(&Config)
			Clear(3, nil, nil)
		}
	}
}

/*
--------------------Sniper--------------------
*/
func aptos_sniper(Config *config_struct) {

	for {
		Clear(4, "action > aptos sniper", "info")

		menu := climenu.NewButtonMenu("", "Choose marketplace")
		menu.AddMenuItem("Topaz", "topaz_sniper")
		menu.AddMenuItem("BlueMove", "bluemove_sniper")

		action, escaped := menu.Run()
		if escaped {
			return
		}

		switch action {
		case "topaz_sniper":
			topaz_sniper(Config)
		case "bluemove_sniper":
			bluemove_sniper(Config)
		}
	}
}

/*
----------Topaz----------
*/
func topaz_sniper(Config *config_struct) {

	Clear(4, "action > aptos sniper > topaz", "info")

	var collection_info collection_info_struct

	// dump config collection to collection_info
	config_collection, _ := json.Marshal(Config.Collection.Topaz)
	json.Unmarshal(config_collection, &collection_info)

	if collection_info.Name == "" ||
		collection_info.ID == "" ||
		collection_info.Creator == "" {

		collection_name := climenu.GetText("Topaz collection", "eg: Bruh-Bears-43ec2cb158")

		if err := collection_info.topaz_get_collection_id(collection_name); err != nil {
			color.Warn.Tips(err.Error())
			fmt.Scanln()
			return
		}

		Config.Collection.Topaz.ID = collection_info.ID
		Config.Collection.Topaz.Name = collection_info.Name
		Config.Collection.Topaz.Creator = collection_info.Creator

		if err := Config.dump_config(); err != nil {
		}

		Clear(1, nil, nil)
	} else {
		menu := climenu.NewButtonMenu("", "Use last sniped collection ["+collection_info.Name+"]")
		menu.AddMenuItem("Yes", "true")
		menu.AddMenuItem("No", "false")

		use_last_collection, escaped := menu.Run()
		if escaped {
			return
		}

		Clear(3, nil, nil)

		if use_last_collection == "false" {

			collection_name := climenu.GetText("Topaz collection", "eg: Bruh-Bears-43ec2cb158")

			if err := collection_info.topaz_get_collection_id(collection_name); err != nil {
				color.Warn.Tips(err.Error())
				fmt.Scanln()
				return
			}

			Config.Collection.Topaz.ID = collection_info.ID
			Config.Collection.Topaz.Name = collection_info.Name
			Config.Collection.Topaz.Creator = collection_info.Creator

			if err := Config.dump_config(); err != nil {
			}

			Clear(1, nil, nil)
		}
	}

	var sniped_price float64
	var err error

	fmt.Printf("%s %s\n", color.Magenta.Text("Collection"), collection_info.Name)
	for {
		input := climenu.GetText("Max price sniped", "eg: 0.5")
		sniped_price, err = strconv.ParseFloat(input, 64)
		Clear(1, nil, nil)
		if err == nil {
			fmt.Printf("%s %f \n", color.Magenta.Text("Max price "), sniped_price)
			sniped_price = sniped_price * 100_000_000
			break
		}
	}

	// create new workspace for sniper in terminal
	term.Init()
	defer term.Close()

	logo(Config.wallet.balance)
	Clear(0, "action > aptos sniper > topaz", "info")
	fmt.Printf("%s %s\n", color.Magenta.Text("Collection"), collection_info.Name)
	fmt.Printf("%s %f\n", color.Magenta.Text("Max price "), sniped_price/100_000_000)

	fmt.Printf("[%s] [%s] %s\n",
		color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
		color.Yellow.Text("INFO   "),
		"Start topaz sniper",
	)

	sniper := func(escaped *bool) {
		url := fmt.Sprintf("https://api-v1.topaz.so/api/listing-view-p?collection_id=%s&from=0&to=49&sort_mode=PRICE_LOW_TO_HIGH&buy_now=false&page=0&min_price=undefined&max_price=null&filters={}&search=null", collection_info.ID)
		req, _ := http.NewRequest("GET", url, nil)
		// try mint list
		var try_buy_nft []string
		var response topaz_listing_struct

		// start sniper
		for !*escaped {
			res, err := http.DefaultClient.Do(req)

			if *escaped {
				break
			}

			if err != nil {
				// error send request
				fmt.Printf("[%s] [%s] %s\n",
					color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
					color.Red.Text("ERROR  "),
					func() string {
						if res.StatusCode == 429 {
							return "topaz: 429 Too many requests"
						} else {
							return fmt.Sprintf("topaz: %s", res.Status)
						}
					}(),
				)

				time.Sleep(10000 * time.Millisecond)

				continue
			}

			defer res.Body.Close()

			var body []byte
			if body, err = ioutil.ReadAll(res.Body); err != nil {
				fmt.Printf("[%s] [%s] %s\n",
					color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
					color.Red.Text("ERROR  "),
					color.Red.Text("Monitor: Error get body"),
				)
				time.Sleep(2)

				continue
			}

			if err = json.Unmarshal(body, &response); err != nil {
				fmt.Printf("[%s] [%s] %s\n",
					color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
					color.Red.Text("ERROR  "),
					color.Red.Text("Error get response data"),
				)

				time.Sleep(10000 * time.Millisecond)

				continue
			}

			for _, listing := range response.Data {

				if listing.Price <= sniped_price {

					in_try_buy_nft := func(list []string, str string) bool {
						for _, v := range list {
							if v == str {
								return true
							}
						}

						return false
					}(try_buy_nft, listing.UpdatedAT)

					if !in_try_buy_nft {

						go send_transaction(
							Config,

							payload_struct{
								Type:     "entry_function_payload",
								Function: "0x2c7bccf7b31baf770fdbcc768d9e9cb3d87805e255355df5db32ac9a669010a2::marketplace_v2::buy",
								TypeArguments: []string{
									"0x1::aptos_coin::AptosCoin",
								},
								Arguments: []string{
									listing.Seller,
									fmt.Sprintf("%d", int(listing.Price)),
									"1",
									collection_info.Creator,
									collection_info.Name,
									listing.TokenName,
									"0",
								},
							},

							nft_info{
								token_name: listing.TokenName,
								price:      listing.Price,
								rank:       1,
								image:      listing.PreviewURI,
								mode:       "sniper",
							},
						)

						fmt.Printf("[%s] [%s] %s\n",
							color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
							color.Yellow.Text("INFO   "),
							fmt.Sprintf("New item found for %f Apt", (listing.Price/100_000_000)),
						)

						try_buy_nft = append(try_buy_nft, listing.UpdatedAT)
						time.Sleep(100 * time.Millisecond)
					}
				}
			}
			// cool down
			time.Sleep(1000 * time.Millisecond)
		}
	}

	var escaped bool
	go sniper(&escaped)

	for {
		switch ev := term.PollEvent(); ev.Type {
		case term.EventKey:
			switch ev.Key {
			case term.KeyEsc:
				escaped = true

				time.Sleep(2000 * time.Millisecond)

				fmt.Printf("[%s] [%s] %s\n",
					color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
					color.Green.Text("INFO   "),
					"Sniper stopped",
				)

				term.Close()
				return
			}
		}
	}
}

func (collection_info *collection_info_struct) topaz_get_collection_id(collection_name string) error {

	req, _ := http.NewRequest("GET", "https://api-v1.topaz.so/api/collection?slug="+collection_name, nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.New("error get collection id. Press enter for back.")
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.New("error get body. Press enter for back.")
	}

	var response struct {
		Data struct {
			Collection struct {
				Collection_id string `json:"collection_id"`
				Creator       string `json:"creator"`
				Name          string `json:"name"`
			} `json:"collection"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &response); (err != nil) || (response.Data.Collection.Collection_id == "") {
		return errors.New("error response body. Press enter for back.")
	}

	collection_info.Name = response.Data.Collection.Name
	collection_info.ID = url.PathEscape(response.Data.Collection.Collection_id)
	collection_info.Creator = response.Data.Collection.Creator

	// success get collection_id
	return nil
}

/*
----------Bluemove----------
*/
func bluemove_sniper(Config *config_struct) {

	Clear(4, "action > aptos sniper > bluemove", "info")

	var collection_info collection_info_struct

	// dump config collection to collection_info
	config_collection, _ := json.Marshal(Config.Collection.Bluemove)
	json.Unmarshal(config_collection, &collection_info)

	if collection_info.ID == "" || collection_info.Creator == "" || collection_info.Name == "" {

		collection_info.ID = climenu.GetText("Bluemove collection", "eg: bruh-bears")

		if err := collection_info.bluemove_get_collection_id(collection_info.ID); err != nil {
			color.Warn.Tips(err.Error())
			fmt.Scanln()
			return
		}

		Config.Collection.Bluemove.Name = collection_info.Name
		Config.Collection.Bluemove.ID = collection_info.ID
		Config.Collection.Bluemove.Creator = collection_info.Creator

		if err := Config.dump_config(); err != nil {
		}

		Clear(1, nil, nil)
	} else {
		menu := climenu.NewButtonMenu("", "Use last sniped collection ["+Config.Collection.Bluemove.Name+"]")
		menu.AddMenuItem("Yes", "true")
		menu.AddMenuItem("No", "false")

		use_last_collection, escaped := menu.Run()
		if escaped {
			return
		}

		Clear(3, nil, nil)

		if use_last_collection == "false" {

			collection_info.ID = climenu.GetText("Bluemove collection", "eg: bruh-bears")

			if err := collection_info.bluemove_get_collection_id(collection_info.ID); err != nil {
				color.Warn.Tips(err.Error())
				fmt.Scanln()
				return
			}

			Config.Collection.Bluemove.Name = collection_info.Name
			Config.Collection.Bluemove.ID = collection_info.ID
			Config.Collection.Bluemove.Creator = collection_info.Creator

			if err := Config.dump_config(); err != nil {
			}

			Clear(1, nil, nil)
		}
	}

	var sniped_price float64
	var err error

	fmt.Printf("%s %s\n", color.Magenta.Text("Collection"), collection_info.Name)
	for {
		input := climenu.GetText("Max price sniped", "eg: 0.5")
		sniped_price, err = strconv.ParseFloat(input, 64)
		Clear(1, nil, nil)
		if err == nil {
			fmt.Printf("%s %f \n", color.Magenta.Text("Max price "), sniped_price)
			sniped_price = sniped_price * 100_000_000
			break
		}
	}

	// create new workspace for sniper in terminal
	term.Init()
	defer term.Close()

	logo(Config.wallet.balance)
	Clear(0, "action > aptos sniper > bluemove", "info")
	fmt.Printf("%s %s\n", color.Magenta.Text("Collection"), collection_info.Name)
	fmt.Printf("%s %f\n", color.Magenta.Text("Max price "), sniped_price/100_000_000)

	fmt.Printf("[%s] [%s] %s\n",
		color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
		color.Yellow.Text("INFO   "),
		"Start bluemove sniper",
	)

	sniper := func(escaped *bool) {
		url := fmt.Sprintf("https://aptos-mainnet-api.bluemove.net/api/market-items?filters[collection][slug][$eq]=%s&filters[status][$eq]=1&filters[price][$gte]=0&filters[price][$lte]=%d&sort[0]=price:asc&pagination[page]=1&pagination[pageSize]=5", collection_info.ID, int(sniped_price))

		// try mint list
		var try_buy_nft []string
		var response bluemove_listing_struct

		// start sniper
		for !*escaped {
			req, _ := http.NewRequest("GET", url, nil)
			res, err := http.DefaultClient.Do(req)

			if *escaped {
				break
			}

			if err != nil {
				// error send request
				fmt.Printf("[%s] [%s] %s\n",
					color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
					color.Red.Text("ERROR  "),
					color.Red.Text(func() string {
						if res.StatusCode == 429 {
							return "bluemove: 429 Too many requests"
						} else {
							return fmt.Sprintf("bluemove status: %s", res.Status)
						}
					}()),
				)

				time.Sleep(10000 * time.Millisecond)

				continue
			}
			defer res.Body.Close()

			var body []byte
			if body, err = ioutil.ReadAll(res.Body); err != nil {
				// error read body
				fmt.Printf("[%s] [%s] %s\n",
					color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
					color.Red.Text("ERROR  "),
					color.Red.Text("error get body"),
				)

				time.Sleep(10000 * time.Millisecond)

				return
			}

			if err = json.Unmarshal(body, &response); err != nil {
				fmt.Printf("[%s] [%s] %s\n",
					color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
					color.Red.Text("ERROR  "),
					color.Red.Text("Error decoding response body"),
				)

				time.Sleep(5000 * time.Millisecond)

				continue
			}

			for _, listing := range response.Data {
				if listing.Attributes.Price <= sniped_price {

					in_try_buy_nft := func(list []string, str string) bool {
						for _, v := range list {
							if v == str {
								return true
							}
						}

						return false
					}(try_buy_nft, listing.Attributes.UpdatedAt)

					if !in_try_buy_nft {

						go send_transaction(
							Config,

							payload_struct{
								Function:      "0xd1fd99c1944b84d1670a2536417e997864ad12303d19eac725891691b04d614e::marketplaceV2::batch_buy_script",
								TypeArguments: []string{},
								Arguments: [][]string{
									{
										collection_info.Creator,
									},
									{
										collection_info.Name,
									},
									{
										listing.Attributes.Name,
									},
									{
										fmt.Sprintf("%d0", int(listing.Attributes.Price)),
									},
								},
								Type: "entry_function_payload",
							},

							nft_info{
								token_name: listing.Attributes.Name,
								price:      listing.Attributes.Price,
								rank:       listing.Attributes.Rank,
								image:      listing.Attributes.URIMedia,
								mode:       "sniper",
							},
						)

						fmt.Printf("[%s] [%s] %s\n",
							color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
							color.Yellow.Text("INFO   "),
							fmt.Sprintf("New item found for %f Apt", (listing.Attributes.Price/100_000_000)),
						)

						try_buy_nft = append(try_buy_nft, listing.Attributes.UpdatedAt)
						time.Sleep(100 * time.Millisecond)
					}
				}
			}
			// cool down
			time.Sleep(1000 * time.Millisecond)
		}
	}

	var escaped bool
	go sniper(&escaped)

	for {
		switch ev := term.PollEvent(); ev.Type {
		case term.EventKey:
			switch ev.Key {
			case term.KeyEsc:
				escaped = true

				time.Sleep(2000 * time.Millisecond)

				fmt.Printf("[%s] [%s] %s\n",
					color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
					color.Green.Text("INFO   "),
					"Sniper stopped",
				)

				term.Close()
				return
			}
		}
	}
}

func (collection_info *collection_info_struct) bluemove_get_collection_id(collection_id string) error {

	url := "https://aptos-mainnet-api.bluemove.net/api/collections?sort[0]=total_volume:desc&pagination[page]=1&pagination[pageSize]=10000"

	req, _ := http.NewRequest("GET", url, nil)
	res, err := http.DefaultClient.Do(req)

	if err != nil {
		return errors.New("bluemove: response error. Press enter for back.")
	}

	defer res.Body.Close()

	var body []byte
	if body, err = ioutil.ReadAll(res.Body); err != nil {
		return errors.New("error read response body. Press enter for back.")
	}

	var response bluemove_collections_struct
	if err = json.Unmarshal(body, &response); err != nil {
		return errors.New("response decode error. Press enter for back.")
	}

	if len(response.Data) == 0 {
		return errors.New("response body empty. Press enter for back.")
	}

	for _, collection := range response.Data {
		if collection.Attributes.Slug == collection_id {
			collection_info.Name = collection.Attributes.Name
			collection_info.ID = collection.Attributes.Slug
			collection_info.Creator = collection.Attributes.Creator
			return nil
		}
	}

	return errors.New("error found collection. Press enter for back.")
}

/*
--------------------Transactions--------------------
*/
func send_transaction(Config *config_struct, payload payload_struct, nft_info nft_info) {

	thx := map[string]interface{}{
		"sender":                    Config.wallet.address_str,
		"sequence_number":           fmt.Sprintf("%d", Config.wallet.sequence_number),
		"max_gas_amount":            "100000",
		"gas_unit_price":            "100",
		"expiration_timestamp_secs": fmt.Sprintf("%d", time.Now().Unix()+600),
		"payload":                   payload,
		"signature":                 nil,
	}

	txn_request, _ := json.Marshal(thx)
	req, _ := http.NewRequest("POST", Config.client.encode, bytes.NewReader(txn_request))
	req.Header.Add("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if (err != nil) || (res.StatusCode != 200) {
		fmt.Printf("[%s] [%s] %s\n",
			color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
			color.Red.Text("ERROR  "),
			color.Red.Text("Error encode submission"),
		)

		return
	}
	defer res.Body.Close()

	var body []byte
	if body, err = ioutil.ReadAll(res.Body); err != nil {
		fmt.Printf("[%s] [%s] %s\n",
			color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
			color.Red.Text("ERROR  "),
			color.Red.Text("Error read body"),
		)

		return
	}

	to_sign := string(body)[3:]
	to_sign = to_sign[:len(to_sign)-1]

	data, _ := hex.DecodeString(to_sign)

	signature := ed25519.Sign(Config.wallet.privateKey, data)
	thx["signature"] = map[string]string{
		"type":       "ed25519_signature",
		"public_key": Config.wallet.publicKeyStr,
		"signature":  fmt.Sprintf("0x%x", signature),
	}

	txn_request, _ = json.Marshal(thx)

	req, _ = http.NewRequest("POST", Config.client.transactions, bytes.NewReader(txn_request))
	req.Header.Add("Content-Type", "application/json")

	res, err = http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("[%s] [%s] %s\n",
			color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
			color.Red.Text("ERROR  "),
			color.Red.Text("Error send transaction"),
		)

		return
	}
	defer res.Body.Close()

	if body, err = ioutil.ReadAll(res.Body); err != nil {
		fmt.Printf("[%s] [%s] %s\n",
			color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
			color.Red.Text("ERROR  "),
			color.Red.Text("Error read body"),
		)
	}

	switch res.StatusCode {
	case 202:
		var response struct {
			Hash string `json:"hash"`
		}

		if json.Unmarshal(body, &response); (err != nil) || (response.Hash == "") {
			fmt.Printf("[%s] [%s] %s\n",
				color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
				color.Red.Text("ERROR  "),
				color.Red.Text("Transaction error"),
			)

			return
		}

		fmt.Printf("[%s] [%s] %s\n",
			color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
			color.Yellow.Text("INFO   "),
			color.Yellow.Text("Transaction send successfully thx: "+response.Hash),
		)

		Config.wallet.sequence_number += 1
		time.Sleep(10000 * time.Millisecond)

		req, _ := http.NewRequest("GET", Config.client.result+response.Hash, nil)
		res, err := http.DefaultClient.Do(req)
		if (err != nil) || (res.StatusCode != 200) {
			fmt.Printf("[%s] [%s] %s\n",
				color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
				color.Red.Text("ERROR  "),
				color.Red.Text("Faild purchased"),
			)

			return
		}
		defer res.Body.Close()

		var body []byte
		if body, err = ioutil.ReadAll(res.Body); err != nil {
			fmt.Printf("[%s] [%s] %s\n",
				color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
				color.Red.Text("ERROR  "),
				color.Red.Text("Error read response body"),
			)

			return
		}

		var response_hush struct {
			Success   bool   `json:"success"`
			Message   string `json:"message"`
			Vm_status string `json:"vm_status"`
		}
		json.Unmarshal(body, &response_hush)

		if response_hush.Message != "" {
			fmt.Printf("[%s] [%s] %s\n",
				color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
				color.Red.Text("ERROR  "),
				color.Yellow.Text(response_hush.Message),
			)

			return
		}

		if response_hush.Success == true {
			fmt.Printf("[%s] [%s] %s\n",
				color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
				color.Green.Text("SUCCESS"),
				func() string {
					switch nft_info.mode {
					case "sniper":
						return color.Green.Text("Successfully purchased " + nft_info.token_name + " for " + fmt.Sprintf("%f", nft_info.price/100_000_000))
					default:
						return color.Green.Text("Successfully purchased")
					}
				}(),
			)

			return
		} else {
			fmt.Printf("[%s] [%s] %s\n",
				color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
				color.Red.Text("ERROR  "),
				color.Red.Text("Faild purchased: "+response_hush.Vm_status),
			)
		}

	case 400:
		var response struct {
			Message string `json:"message"`
		}

		json.Unmarshal(body, &response)

		fmt.Printf("[%s] [%s] %s\n",
			color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
			color.Red.Text("ERROR  "),
			color.Red.Text(response.Message),
		)
		if response.Message == "Invalid transaction: Type: Validation Code: SEQUENCE_NUMBER_TOO_OLD" {
			Config.wallet.sequence_number += 1
		}

	default:
		fmt.Printf("[%s] [%s] %s\n",
			color.Magenta.Text(strings.Split(time.Now().String(), " ")[1][:12]),
			color.Red.Text("ERROR  "),
			color.Red.Text("Error send transaction: "+res.Status),
		)
	}
}

/*
--------------------Settings--------------------
*/
func settings(Config *config_struct) {

	for {
		Clear(4, "action > settings", "info")

		menu := climenu.NewButtonMenu("", "Choose action")
		menu.AddMenuItem("Discord hook", "discord_hook")

		action, escaped := menu.Run()
		if escaped {
			return
		}

		switch action {
		case "discord_hook":
			settings_discord_hook(Config)
		}
	}
}

func settings_discord_hook(Config *config_struct) {

	Clear(3, "action > settings > discord hook", "info")

	for {
		menu := climenu.NewButtonMenu("", "Choose action")
		menu.AddMenuItem("Change hook", "change_hook")
		menu.AddMenuItem("Send fail purchase", "send_fail")

		action, escaped := menu.Run()
		if escaped {
			return
		}

		switch action {
		case "change_hook":
			Clear(4, "action > settings > discord hook > change", "info")
			Config.Discord.Hook = climenu.GetText("Discord hook", "eg: https://discord.com/api/webhooks/...")
			Clear(2, "action > settings > discord hook", "info")

		case "send_fail":
			Clear(4, "action > settings > discord hook > send fail", "info")
			menu := climenu.NewButtonMenu("", "Send fail purchase")
			menu.AddMenuItem("Yes", "true")
			menu.AddMenuItem("No", "false")

			action, escaped := menu.Run()
			Clear(4, "action > settings > discord hook", "info")
			if escaped {
				continue
			}
			switch action {
			case "true":
				Config.Discord.Send_fail = true
			case "false":
				Config.Discord.Send_fail = false
			}
		}
		if err := Config.dump_config(); err != nil {
		}
	}
}

/*
--------------------Config--------------------
*/
func (config *config_struct) load_config() error {

	var file *os.File

	// get path
	path, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return err
	}

	// try open config file
	file, err = os.OpenFile(fmt.Sprintf("%s/config.json", path), os.O_CREATE, os.ModePerm)
	defer file.Close()
	if err != nil {
		return err
	}

	// crerate config
	byteValue, _ := ioutil.ReadAll(file)
	if (string(byteValue)) == "" {
		color.Info.Tips("successfully created config.json")

		config.Node = "https://fullnode.mainnet.aptoslabs.com/v1"

		js, _ := json.MarshalIndent(config, "", "  ")
		if err = ioutil.WriteFile(fmt.Sprintf("%s/config.json", path), js, os.ModePerm); err != nil {
			return errors.New("error write config struct to file")
		}

		return errors.New("need to add data to config.json file")
	}

	// load config to struct
	json.Unmarshal(byteValue, config)

	// check node
	if new_node(config) {
		return errors.New("error node")
	}

	// check wallet
	if new_account(config) {
		return errors.New("wrong private key")
	}

	// check hook
	if config.Discord.Hook == "" {
		return errors.New("discord hook not found")
	}

	// success load config
	return nil
}

func (Config *config_struct) dump_config() error {

	path, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return err
	}

	js, _ := json.MarshalIndent(Config, "", "  ")
	if err = ioutil.WriteFile(fmt.Sprintf("%s/config.json", path), js, os.ModePerm); err != nil {
		return errors.New("error dump config")
	}

	return nil
}

func new_node(Config *config_struct) bool {

	req, _ := http.NewRequest("GET", Config.Node, nil)
	_, err := http.DefaultClient.Do(req)

	if err != nil {
		return true
	}

	Config.client = struct {
		url          string
		accounts     string
		encode       string
		transactions string
		result       string
	}{
		url:          Config.Node,
		accounts:     Config.Node + "/accounts/",
		encode:       Config.Node + "/transactions/encode_submission",
		transactions: Config.Node + "/transactions",
		result:       Config.Node + "/transactions/by_hash/",
	}

	return false
}

func new_account(Config *config_struct) bool {

	switch len(Config.Key) {
	case 64:
	case 66:
		Config.Key = Config.Key[2:]
	default:
		return true
	}

	seed, _ := hex.DecodeString(Config.Key)

	privateKey := ed25519.NewKeyFromSeed(seed[:])

	publicKey := privateKey.Public().(ed25519.PublicKey)

	data := append(publicKey, 0x00)
	authKey := sha3.Sum256(data)

	Config.wallet = struct {
		balance         string
		privateKey      ed25519.PrivateKey
		publicKey       ed25519.PublicKey
		address         [32]byte
		privateKeyStr   string
		publicKeyStr    string
		address_str     string
		sequence_number int
	}{
		privateKey:    privateKey,
		publicKey:     publicKey,
		address:       authKey,
		privateKeyStr: fmt.Sprintf("0x%x", privateKey),
		publicKeyStr:  fmt.Sprintf("0x%x", publicKey),
		address_str:   fmt.Sprintf("0x%x", authKey),
	}

	Config.client.accounts += Config.wallet.address_str

	// get balance
	req, _ := http.NewRequest("GET", Config.client.accounts+"/resources", nil)
	req.Header.Add("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return true
	}
	defer res.Body.Close()

	var body []byte
	if body, err = ioutil.ReadAll(res.Body); err != nil {
		return true
	}

	type response_balance []struct {
		Type string `json:"type"`
		Data struct {
			Coin struct {
				Value float64 `json:"value,string"`
			} `json:"coin"`
		} `json:"data,omitempty"`
	}

	var response_resources response_balance
	json.Unmarshal(body, &response_resources)

	Config.wallet.balance = fmt.Sprintf("%f", response_resources[3].Data.Coin.Value/100_000_000)

	// get sequence number
	req, _ = http.NewRequest("GET", Config.client.accounts, nil)
	req.Header.Add("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		return true
	}
	defer res.Body.Close()
	body, _ = ioutil.ReadAll(res.Body)

	var response struct {
		Sequence_number string `json:"sequence_number"`
	}
	json.Unmarshal(body, &response)

	Config.wallet.sequence_number, _ = strconv.Atoi(response.Sequence_number)

	return false
}

/*
--------------------CLI--------------------
*/

func logo(Balance string) {

	color.Magentaln(`by https://github.com/mkdr4
  __    __     ______     ______     ______     __  __     ______     __  __
 /\ "-./  \   /\  ___\   /\  == \   /\  ___\   /\ \/\ \   /\  == \   /\ \_\ \
 \ \ \-./\ \  \ \  __\   \ \  __<   \ \ \____  \ \ \_\ \  \ \  __<   \ \____ \
  \ \_\ \ \_\  \ \_____\  \ \_\ \_\  \ \_____\  \ \_____\  \ \_\ \_\  \/\_____\
   \/_/  \/_/   \/_____/   \/_/ /_/   \/_____/   \/_____/   \/_/ /_/   \/` + Version + `/		
Balance: ` + Balance)
}

func Clear(count_line int, info any, type_info any) {

	for i := 0; i < count_line; i++ {
		fmt.Print("\033[1A\033[K")
	}

	if info != nil {
		switch type_info {
		case "info":
			color.Grayln(info)
		case "error":
			color.Redln(info)
		}
	}
}
