package k8s

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"chain4travel.com/camktncr/pkg/version1"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

const DEFAULT_PENDING_TIME_OFFSET = 2 * time.Minute
const SYNC_BOUND = time.Minute

func RegisterValidators(ctx context.Context, restClient *rest.Config, k8sConfig version1.K8sConfig, stakers []version1.Staker, allowError bool) error {
	roundTripper, upgrader, err := spdy.RoundTripperFor(restClient)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", k8sConfig.Namespace, fmt.Sprintf("%s-root-0", k8sConfig.K8sPrefix))
	hostIP := strings.TrimLeft(restClient.Host, "htps:/")
	serverURL := url.URL{Scheme: "https", Path: path, Host: hostIP}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, &serverURL)

	stopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)

	forwarder, err := portforward.New(dialer, []string{"9650"}, stopChan, readyChan, out, errOut)
	if err != nil {
		return err
	}

	go func() {
		for range readyChan { // Kubernetes will close this channel when it has something to tell us.
		}
		if len(errOut.String()) != 0 {
			panic(errOut.String())
		} else if len(out.String()) != 0 {
			fmt.Println(out.String())
		}
	}()

	go func() {
		if err = forwarder.ForwardPorts(); err != nil { // Locks until stopChan is closed.
			fmt.Println(err)
		}
	}()
	time.Sleep(1 * time.Second) // waiting for the connection to open kinda hacky ngl
	defer close(stopChan)

	for {
		err := isBootstrapped()
		if err != nil {
			log.Println("root has not bootstrapped yet")
		} else {
			break
		}
		time.Sleep(DEFAULT_TIMEOUT)
	}

	g, ctx := errgroup.WithContext(ctx)

	for _, staker := range stakers {
		g.Go(func() error {
			err := registerValidator(ctx, staker, allowError)
			if err != nil {
				return err
			}
			return nil
		})
		time.Sleep(1 * time.Second)
	}

	err = g.Wait()
	if err != nil {
		return err
	}

	return nil
}

type ResultResp struct {
	Result struct {
		TxID   string `json:"txID"`
		Status string `json:"status"`
	} `json:"result"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

var errNotAddedToMempool = errors.New("tx was not added to mempool")

func verifyStatus(ctx context.Context, staker version1.Staker, txId string) error {

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("could not wait for validator %s: Reason: %v", staker.NodeID, ctx.Err())
		default:
			getTxStatusPayload := strings.NewReader(fmt.Sprintf(`{
				"jsonrpc":"2.0",
				"id"     :1,
				"method" :"platform.getTxStatus",
				"params" :{
					"txID":"%s",
					"includeReason": true
				}
			}`, txId))

			res, err := http.Post("http://localhost:9650/ext/bc/P", "application/json", getTxStatusPayload)
			if err != nil {
				return err
			}

			body, err := ioutil.ReadAll(res.Body)
			if err != nil {
				return err
			}

			fmt.Println(string(body))

			var txStatus ResultResp
			err = json.Unmarshal(body, &txStatus)
			if err != nil {
				return err
			}

			fmt.Printf("%s: TXID %s Status: %s\n", staker.NodeID, txId, txStatus.Result.Status)

			switch txStatus.Result.Status {
			case "Committed":
				return nil
			case "Unknown", "Dropped":
				return errNotAddedToMempool
			}

			time.Sleep(DEFAULT_TIMEOUT)
		}
	}
}

func waitForValidatorToBecomeActive(ctx context.Context, staker version1.Staker) error {
	for {

		select {
		case <-ctx.Done():
			return fmt.Errorf("could not wait for validator %s to become active. Reason: %v", staker.NodeID, ctx.Err())
		default:
			active, err := isActiveValidator(staker)
			if err != nil {
				return err
			}

			if active {
				return nil
			}

			fmt.Printf("validator %s not active yet\n", staker.NodeID)
			time.Sleep(DEFAULT_PENDING_TIME_OFFSET / 10)

		}

	}
}

func isActiveValidator(staker version1.Staker) (bool, error) {
	getCurrentValidatorsPayload := strings.NewReader(`{
			"jsonrpc": "2.0",
			"method": "platform.getCurrentValidators",
			"params": {
				"subnetID": null,
				"nodeIDs": []
			},
			"id": 1
		}`)

	res, err := http.Post("http://localhost:9650/ext/bc/P", "application/json", getCurrentValidatorsPayload)
	if err != nil {
		return false, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return false, err
	}
	current := string(body)
	if strings.Contains(current, staker.NodeID) {
		return true, nil
	}

	return false, nil

}

func isPendingValidator(staker version1.Staker) (bool, error) {
	getPendingValidatorsPayload := strings.NewReader(`{
			"jsonrpc": "2.0",
			"method": "platform.getPendingValidators",
			"params": {
				"subnetID": null,
				"nodeIDs": []
			},
			"id": 1
		}`)

	res, err := http.Post("http://localhost:9650/ext/bc/P", "application/json", getPendingValidatorsPayload)
	if err != nil {
		return false, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return false, err
	}
	pending := string(body)
	if strings.Contains(pending, staker.NodeID) {
		return true, nil
	}

	return false, nil

}

func registerValidator(ctx context.Context, staker version1.Staker, allowError bool) error {
	day, err := time.ParseDuration("24h")
	if err != nil {
		return err
	}

	stakeDur := day * 30
	username := staker.PublicAddress
	sum := sha1.Sum([]byte(username))
	password := hex.EncodeToString(sum[:])
	addr := fmt.Sprintf("P-%s", strings.Split(staker.PublicAddress, "-")[1])

	createUserPostData := strings.NewReader(fmt.Sprintf(`{
			"jsonrpc":"2.0",
			"id"     :1,
			"method" :"keystore.createUser",
			"params" :{
				"username": "%s",
				"password": "%s"
			}
		}`, username, password))
	res, err := http.Post("http://localhost:9650/ext/keystore", "application/json", createUserPostData)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	fmt.Println(string(body))

	importKeyPostData := strings.NewReader(fmt.Sprintf(`{
			"jsonrpc":"2.0",
			"id"     :1,
			"method" :"platform.importKey",
			"params" :{
				"username":"%s",
				"password":"%s",
				"privateKey":"%s"
			}
		}`, username, password, staker.PrivateKey))
	res, err = http.Post("http://localhost:9650/ext/bc/P", "application/json", importKeyPostData)
	if err != nil {
		return err
	}

	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	fmt.Println(string(body))

	count := 0
	startTime := time.Now().Add(DEFAULT_PENDING_TIME_OFFSET + SYNC_BOUND)
	endTime := startTime.Add(stakeDur)

	for {
		count++
		fmt.Printf("Attempt %d: %s\n", count, staker.NodeID)

		active, err := isActiveValidator(staker)
		if err != nil {
			return err
		}

		if active {
			return nil
		}

		pending, err := isPendingValidator(staker)
		if err != nil {
			return err
		}

		if pending {
			return waitForValidatorToBecomeActive(ctx, staker)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("could not add %s as a validator: %v", staker.NodeID, ctx.Err())
		default:
			if time.Now().After(startTime) {
				fmt.Println("XXXXXX")
				startTime = time.Now().Add(DEFAULT_PENDING_TIME_OFFSET + SYNC_BOUND)
			}
			addVaidatorPostData := strings.NewReader(fmt.Sprintf(`{
			"jsonrpc":"2.0",
			"id"     :1,
			"method": "platform.addValidator",
			"params": {
				"nodeID":"%s",
				"startTime": %d,
				"endTime": %d,
				"stakeAmount": %d,
				"rewardAddress": "%s",
				"delegationFeeRate": 10,
				"username": "%s",
				"password": "%s"
			}
		}`, staker.NodeID, startTime.Unix(), endTime.Unix(), staker.Stake, addr, username, password))
			println(addVaidatorPostData)
			res, err = http.Post("http://localhost:9650/ext/bc/P", "application/json", addVaidatorPostData)
			if err != nil {
				return err
			}

			body, err = ioutil.ReadAll(res.Body)
			if err != nil {
				return err
			}

			var result ResultResp
			err = json.Unmarshal(body, &result)
			if err != nil {
				return err
			}

			txId := result.Result.TxID
			err = fmt.Errorf("failed to add validator %s - Reason: %s", staker.NodeID, result.Error.Message)
			if txId == "" {
				if allowError {
					fmt.Println("Ignoring error:", err)
					time.Sleep(DEFAULT_TIMEOUT)
					continue
				} else {
					return err
				}
			}

			time.Sleep(DEFAULT_TIMEOUT)

			err = verifyStatus(ctx, staker, txId)
			if err != nil {
				if err == errNotAddedToMempool {
					continue
				}
				return err
			}

			time.Sleep(DEFAULT_TIMEOUT)

			err = waitForValidatorToBecomeActive(ctx, staker)
			if err != nil {
				return err
			}

			return nil

		}
	}

}

func isBootstrapped() error {
	url := "http://localhost:9650/ext/info"
	method := "POST"

	payload := strings.NewReader(`{
    "jsonrpc":"2.0",
    "id"     :1,
    "method" :"info.isBootstrapped",
    "params": {
        "chain": "P"
    }
}`)
	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	parsed := make(map[string]interface{})

	json.Unmarshal(body, &parsed)

	result := parsed["result"].(map[string]interface{})

	if result["isBootstrapped"] != true {
		return fmt.Errorf("not bootstrapped yet")
	}

	return nil
}
