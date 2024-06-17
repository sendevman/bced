package routes

import (
	"bytes"
	"encoding/hex"
	"encoding/json"

	"github.com/btcsuite/btcd/btcec"

	"github.com/deso-protocol/core/lib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"net/http"
	"net/http/httptest"
	"testing"
)

func signTransaction(t *testing.T, txn *lib.MsgDeSoTxn) {
	privKeyBytes, _, err := lib.Base58CheckDecode(senderPrivString)
	require.NoError(t, err)
	privKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), privKeyBytes)
	txnSignature, err := txn.Sign(privKey)
	require.NoError(t, err)
	txn.Signature.SetSignature(txnSignature)
}

func SignAndSubmitTransaction(t *testing.T, privateKeyBase58Check string, txn *lib.MsgDeSoTxn, apiServer *APIServer) *SubmitTransactionResponse {
	t.Helper()
	assert := assert.New(t)
	signTransaction(t, txn)
	txnBytes, err := txn.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	hexTxnBytes := hex.EncodeToString(txnBytes)

	// Compare the expected
	//assert.Equal(&expectedResponse, unmarshalResponse)
	submitReq := &SubmitTransactionRequest{
		TransactionHex: hexTxnBytes,
	}
	requestbody, err := json.Marshal(submitReq)

	if err != nil {
		t.Fatal(err)
	}

	request, _ := http.NewRequest("POST", RoutePathSubmitTransaction, bytes.NewBuffer(requestbody))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	apiServer.router.ServeHTTP(response, request)
	// assert the response status.
	assert.Equal(200, response.Code, "OK response is expected")

	// Deserialize the response.
	unmarshalResponse := &SubmitTransactionResponse{}
	err = json.Unmarshal(response.Body.Bytes(), unmarshalResponse)
	require.NoError(t, err)
	return unmarshalResponse
}

// Function to fetch the access group ID.
func fetchAccessGroupID(t *testing.T, apiServer *APIServer, publicKeyBase58Check string) *GetAccessGroupsResponse {
	t.Helper()
	// form the request for RoutePathGetAllUserAccessGroups
	values := GetAccessGroupsRequest{PublicKeyBase58Check: publicKeyBase58Check}
	requestbody, err := json.Marshal(values)

	routePath := RoutePathGetAllUserAccessGroups
	require.NoError(t, err)
	responseBody := ExecuteRequest(t, apiServer, routePath, requestbody)
	// Deserialize the response.
	unmarshalResponse := &GetAccessGroupsResponse{}
	err = json.Unmarshal(responseBody, unmarshalResponse)
	require.NoError(t, err)
	return unmarshalResponse
}

func ExecuteRequest(t *testing.T, apiServer *APIServer, routePath string, requestBody []byte) []byte {
	t.Helper()
	assert := assert.New(t)

	// Send the post request to fetch access groups of the user.
	request, err := http.NewRequest("POST", routePath, bytes.NewBuffer(requestBody))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	apiServer.router.ServeHTTP(response, request)
	// assert the response status.
	assert.Equal(200, response.Code, "OK response is expected")

	return response.Body.Bytes()
}

// This access group key name is reserved since every user by default belongs to them.
func TestAPIAccessGroupBaseGroupMembership(t *testing.T) {
	assert := assert.New(t)

	apiServer, _, _ := newTestAPIServer(t, "" /*globalStateRemoteNode*/, false)

	// form the request for RoutePathGetAllUserAccessGroups
	values := GetAccessGroupsRequest{PublicKeyBase58Check: senderPkString}
	requestbody, err := json.Marshal(values)

	require.NoError(t, err)
	// Expense response for the call to fetch Access group ID.
	expectedResponse := GetAccessGroupsResponse{
		AccessGroupsOwned: []AccessGroupEntryResponse{
			{
				// The user should be the owner of the default base group().
				AccessGroupOwnerPublicKeyBase58Check: senderPkString,
				AccessGroupKeyName:                   string(lib.AccessKeyNameDecode(lib.BaseGroupKeyName())),
				AccessGroupPublicKeyBase58Check:      senderPkString,
				ExtraData:                            nil,
				AccessGroupMemberEntryResponse:       nil,
			},
		},
	}

	// Send the post request to fetch access groups of the user.
	request, err := http.NewRequest("POST", RoutePathGetAllUserAccessGroups, bytes.NewBuffer(requestbody))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	apiServer.router.ServeHTTP(response, request)
	// assert the response status.
	assert.Equal(200, response.Code, "OK response is expected")

	// Deserialize the response.
	unmarshalResponse := &GetAccessGroupsResponse{}
	err = json.Unmarshal(response.Body.Bytes(), unmarshalResponse)
	if err != nil {
		t.Fatal("Unable to Base58 Check decode the result")
	}
	// Compare the expected
	assert.Equal(&expectedResponse, unmarshalResponse)
}

// generates random public key.
func generateRandomPublicKey(t *testing.T) (publicKeyBytes []byte) {
	t.Helper()
	require := require.New(t)
	randomPrivateKey, err := btcec.NewPrivateKey(btcec.S256())
	require.NoError(err)
	randomPublicKeyBytes := randomPrivateKey.PubKey().SerializeCompressed()
	return randomPublicKeyBytes
}

// Tests the creation of new access group, adding members to them,
// and fetching the access groups of public keys.
// Tests the following routes:
// - RoutePathCreateAccessGroup
// - RoutePathAddAccessGroupMembers
// - RoutePathGetAllUserAccessGroups
// - RoutePathGetAllUserAccessGroupsOwned
// - RoutePathGetAllUserAccessGroupsMemberOnly
func TestAPIAccessGroups(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// random public keys to use for access group publickeys.
	groupPk1 := generateRandomPublicKey(t)
	//groupPk2 := generateRandomPublicKey(t)

	// random public keys to be used a users/members access groups.
	member1 := generateRandomPublicKey(t)
	// values for access group keys.
	groupName1String := "group1"

	apiServer := newTestApiServer(t)

	// Create a request to create an access group.
	values := CreateAccessGroupRequest{
		AccessGroupOwnerPublicKeyBase58Check: senderPkString,
		AccessGroupPublicKeyBase58Check:      lib.Base58CheckEncode(groupPk1, false, apiServer.Params),
		AccessGroupKeyName:                   groupName1String,
		MinFeeRateNanosPerKB:                 apiServer.MinFeeRateNanosPerKB,
		TransactionFees:                      nil,
	}

	requestbody, err := json.Marshal(values)
	require.NoError(err)

	responseBytes := ExecuteRequest(t, apiServer, RoutePathCreateAccessGroup, requestbody)

	// Deserialize the response.
	// Tests the response structure.
	// We validate whether the access group creation was successful by fetching the access groups later.
	unmarshalResponse := &CreateAccessGroupResponse{}
	err = json.Unmarshal(responseBytes, unmarshalResponse)
	require.NoError(err)

	// The previous step was just transaction construction phase.
	// Now, sign and submit the transaction, to execute the transaction.
	// First, fetch the transaction from the response of the transaction construction API.
	txn := unmarshalResponse.Transaction
	// Now sign and submit transaction.
	// The test function fails if the submit transaction fails.
	SignAndSubmitTransaction(t, senderPrivString, txn, apiServer)

	// Now that the transaction is submitted, fetch the AccessGroup IDs and
	// check if the new access group exists.
	// for public key senderPkString, and access group key name "groupName1"
	// Fetch all the access groups for sender Pk String
	actualGroupIDsres := fetchAccessGroupID(t, apiServer, senderPkString)
	// Expected response for the call to fetch Access group ID.
	// Sender Public key (senderPkString) should now own two access groups.
	// One is the default access group, the other is the access group we with key "groupName1".
	expectedResponse := GetAccessGroupsResponse{
		AccessGroupsOwned: []AccessGroupEntryResponse{
			{
				// The user should be the owner of the default base group().
				AccessGroupOwnerPublicKeyBase58Check: senderPkString,
				AccessGroupKeyName:                   "",
				AccessGroupPublicKeyBase58Check:      senderPkString,
			},
			{
				// The user should be the owner of the default base group().
				AccessGroupOwnerPublicKeyBase58Check: senderPkString,
				AccessGroupKeyName:                   groupName1String,
				AccessGroupPublicKeyBase58Check:      lib.Base58CheckEncode(groupPk1, false, apiServer.Params),
			},
		},
	}

	// Assert if the expected response and the actual response are the same.
	assert.Equal(&expectedResponse, actualGroupIDsres)

	// Add member1 as a new member of groupName1.
	accessGroupMember1 := AccessGroupMember{
		AccessGroupMemberPublicKeyBase58Check: lib.Base58CheckEncode(member1, false, apiServer.Params),
		AccessGroupMemberKeyName:              "",
		EncryptedKey:                          string([]byte{1, 2, 3}),
	}
	// Call the API to construct the transaction to add the member.
	memberAdd := &AddAccessGroupMembersRequest{
		AccessGroupOwnerPublicKeyBase58Check: senderPkString,
		AccessGroupKeyName:                   groupName1String,
		AccessGroupMemberList:                []AccessGroupMember{accessGroupMember1},
		MinFeeRateNanosPerKB:                 apiServer.MinFeeRateNanosPerKB,
	}

	requestbody, err = json.Marshal(memberAdd)
	require.NoError(err)
	responseBytes = ExecuteRequest(t, apiServer, RoutePathAddAccessGroupMembers, requestbody)

	// Deserialize the response.
	// Validate the response type upon successful deserialization.
	addMemberResponse := &AddAccessGroupMembersResponse{}
	err = json.Unmarshal(responseBytes, addMemberResponse)
	require.NoError(err)

	// The previous step was just transaction construction phase.
	// Now, sign and submit the transaction, to execute the transaction.
	// First, fetch the transaction from the response of the transaction construction API.
	txn = addMemberResponse.Transaction

	// The test function fails if the submit transaction fails.
	SignAndSubmitTransaction(t, senderPrivString, txn, apiServer)

	// Now that the transaction is submitted, fetch the AccessGroup IDs and
	// check if the new member is add to the access group.
	// Fetch all the access groups for member1.
	actualGroupIDsres = fetchAccessGroupID(t, apiServer, lib.Base58CheckEncode(member1, false, apiServer.Params))
	// Expected response for the call to fetch Access group ID.
	expectedResponse = GetAccessGroupsResponse{
		AccessGroupsOwned: []AccessGroupEntryResponse{
			{
				// The user should be the owner of the default base group().
				AccessGroupOwnerPublicKeyBase58Check: lib.Base58CheckEncode(member1, false, apiServer.Params),
				AccessGroupKeyName:                   "",
				AccessGroupPublicKeyBase58Check:      lib.Base58CheckEncode(member1, false, apiServer.Params),
			},
		},
		AccessGroupsMember: []AccessGroupEntryResponse{
			{
				AccessGroupOwnerPublicKeyBase58Check: senderPkString,
				AccessGroupKeyName:                   groupName1String,
				AccessGroupPublicKeyBase58Check:      lib.Base58CheckEncode(groupPk1, false, apiServer.Params),
				AccessGroupMemberEntryResponse: &AccessGroupMemberEntryResponse{
					AccessGroupMemberPublicKeyBase58Check: lib.Base58CheckEncode(member1, false, apiServer.Params),
					AccessGroupMemberKeyName:              "",
					EncryptedKey:                          string([]byte{1, 2, 3}),
				},
			},
		},
	}
	// validate the actual response with the expected response
	assert.Equal(&expectedResponse, actualGroupIDsres)

	// Test Api to fetch owner only access groups.
	ownerOnlyValues := GetAccessGroupsRequest{PublicKeyBase58Check: senderPkString}

	requestbody, err = json.Marshal(ownerOnlyValues)
	require.NoError(err)
	responseBytes = ExecuteRequest(t, apiServer, RoutePathGetAllUserAccessGroupsOwned, requestbody)
	actualOwnerOnlyResponse := &GetAccessGroupsResponse{}
	err = json.Unmarshal(responseBytes, actualOwnerOnlyResponse)
	require.NoError(err)
	expectedResponse = GetAccessGroupsResponse{
		AccessGroupsOwned: []AccessGroupEntryResponse{
			{
				// The user should be the owner of the default base group().
				AccessGroupOwnerPublicKeyBase58Check: senderPkString,
				AccessGroupKeyName:                   "",
				AccessGroupPublicKeyBase58Check:      senderPkString,
			},
			{
				// The user should be the owner of the default base group().
				AccessGroupOwnerPublicKeyBase58Check: senderPkString,
				AccessGroupKeyName:                   groupName1String,
				AccessGroupPublicKeyBase58Check:      lib.Base58CheckEncode(groupPk1, false, apiServer.Params),
			},
		},
	}

	// validate the actual response with the expected response
	assert.Equal(&expectedResponse, actualOwnerOnlyResponse)

	// Test Api to fetch member only access groups.
	memberOnlyValues := GetAccessGroupsRequest{PublicKeyBase58Check: lib.Base58CheckEncode(member1, false, apiServer.Params)}

	requestbody, err = json.Marshal(memberOnlyValues)
	require.NoError(err)
	responseBytes = ExecuteRequest(t, apiServer, RoutePathGetAllUserAccessGroupsMemberOnly, requestbody)
	actualMemberOnlyResponse := &GetAccessGroupsResponse{}
	err = json.Unmarshal(responseBytes, actualMemberOnlyResponse)
	require.NoError(err)

	expectedResponse = GetAccessGroupsResponse{
		AccessGroupsMember: []AccessGroupEntryResponse{
			{
				AccessGroupOwnerPublicKeyBase58Check: senderPkString,
				AccessGroupKeyName:                   groupName1String,
				AccessGroupPublicKeyBase58Check:      lib.Base58CheckEncode(groupPk1, false, apiServer.Params),
				AccessGroupMemberEntryResponse: &AccessGroupMemberEntryResponse{
					AccessGroupMemberPublicKeyBase58Check: lib.Base58CheckEncode(member1, false, apiServer.Params),
					AccessGroupMemberKeyName:              "",
					EncryptedKey:                          string([]byte{1, 2, 3}),
				},
			},
		},
	}
	// validate the actual response with the expected response
	assert.Equal(&expectedResponse, actualMemberOnlyResponse)
}
