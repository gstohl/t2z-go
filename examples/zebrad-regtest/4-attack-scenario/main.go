// Example 4: Attack Scenario - PCZT Malleation Detection
//
// Demonstrates how verify_before_signing catches malicious modifications:
// - Create a legitimate transaction request
// - Simulate an attacker modifying the PCZT
// - Show verification catching the attack
//
// This is a DEMO showing why verification is critical!
// This example does NOT broadcast any transactions.
//
// Run with: go run ./4-attack-scenario

package main

import (
	"fmt"
	"os"

	t2z "github.com/gstohl/t2z/go"
	"github.com/gstohl/t2z/go/examples/zebrad-regtest/common"
)

func main() {
	fmt.Println()
	fmt.Println("======================================================================")
	fmt.Println("  EXAMPLE 4: ATTACK SCENARIO - PCZT Malleation Detection")
	fmt.Println("======================================================================")
	fmt.Println()

	fmt.Println("WARNING: This demonstrates a security feature!")
	fmt.Println("   This example shows why you MUST verify PCZTs before signing.")
	fmt.Println()

	// Initialize data directory (respects T2Z_DATA_DIR env var)
	common.InitDataDir()

	// Create Zebra client
	client := common.NewZebraClient()

	// Load test data
	testData, err := common.LoadTestData()
	if err != nil {
		common.PrintError("Failed to load test data", err)
		fmt.Println("Please run setup first: go run ./setup")
		os.Exit(1)
	}

	// Addresses
	victimAddress := testData.Transparent.Address
	attackerAddress := "tmBsTi2xWTjUdEXnuTceL7fecEQKeWi4vxA"

	fmt.Println("Scenario Setup:")
	fmt.Printf("  Victim's address: %s\n", victimAddress)
	fmt.Printf("  Legitimate recipient: %s\n", victimAddress)
	fmt.Printf("  Attacker's address: %s\n\n", attackerAddress)

	// Fetch mature coinbase UTXOs
	fmt.Println("Fetching mature coinbase UTXOs...")
	utxos, err := common.GetMatureCoinbaseUtxos(client, common.TEST_KEYPAIR, 5)
	if err != nil {
		common.PrintError("Failed to get UTXOs", err)
		os.Exit(1)
	}

	if len(utxos) < 5 {
		common.PrintError("Insufficient UTXOs", fmt.Errorf("need at least 5 mature UTXOs, got %d", len(utxos)))
		os.Exit(1)
	}

	inputs := utxos[:5]
	var totalInput uint64
	for _, u := range inputs {
		totalInput += u.Amount
	}
	fmt.Printf("  Using %d UTXOs totaling: %s ZEC\n\n", len(inputs), common.ZatoshiToZec(totalInput))

	// SCENARIO 1: Legitimate Transaction
	fmt.Println("======================================================================")
	fmt.Println("  SCENARIO 1: Legitimate Transaction")
	fmt.Println("======================================================================")
	fmt.Println()

	paymentAmount := totalInput / 2 // 50%

	legitimatePayments := []t2z.Payment{
		{Address: victimAddress, Amount: paymentAmount},
	}

	fmt.Println("User creates legitimate payment:")
	fmt.Printf("   Send %s ZEC -> %s...\n\n", common.ZatoshiToZec(paymentAmount), victimAddress[:30])

	legitimateRequest, _ := t2z.NewTransactionRequest(legitimatePayments)
	defer legitimateRequest.Free()

	legitimateRequest.SetTargetHeight(2_500_000)

	fmt.Println("1. Proposing legitimate transaction...")
	pczt, err := t2z.ProposeTransaction(inputs, legitimateRequest)
	if err != nil {
		fmt.Printf("   Failed: %v\n", err)
		return
	}
	fmt.Println("   PCZT created")
	fmt.Println()

	fmt.Println("2. Proving transaction...")
	proved, err := t2z.ProveTransaction(pczt)
	if err != nil {
		fmt.Printf("   Failed: %v\n", err)
		return
	}
	fmt.Println("   Proofs generated")
	fmt.Println()

	fmt.Println("3. Verifying PCZT (BEFORE signing)...")
	err = t2z.VerifyBeforeSigning(proved, legitimateRequest, []t2z.TransparentOutput{})
	if err != nil {
		fmt.Printf("   Verification returned: %v\n", err)
	} else {
		fmt.Println("   VERIFICATION PASSED - Safe to sign!")
	}
	fmt.Println()

	// SCENARIO 2: Attack - Wrong Payment Amount
	fmt.Println("======================================================================")
	fmt.Println("  SCENARIO 2: Attack - Wrong Payment Amount")
	fmt.Println("======================================================================")
	fmt.Println()

	fmt.Println("ATTACK: Attacker intercepts PCZT and creates different request")
	wrongAmount := paymentAmount * 2
	fmt.Printf("   Attacker claims payment is %s ZEC (instead of %s ZEC)\n\n",
		common.ZatoshiToZec(wrongAmount), common.ZatoshiToZec(paymentAmount))

	attackedPayments1 := []t2z.Payment{
		{Address: victimAddress, Amount: wrongAmount},
	}

	maliciousRequest1, _ := t2z.NewTransactionRequest(attackedPayments1)
	defer maliciousRequest1.Free()

	fmt.Println("User verifies PCZT before signing...")
	err = t2z.VerifyBeforeSigning(proved, maliciousRequest1, []t2z.TransparentOutput{})
	if err != nil {
		fmt.Println("   ATTACK DETECTED! Verification failed:")
		fmt.Printf("   Error: %v\n\n", err)
		fmt.Println("   Transaction NOT signed - funds are SAFE!")
	} else {
		fmt.Println("   DANGER: Verification passed (should not happen!)")
	}
	fmt.Println()

	// SCENARIO 3: Attack - Wrong Recipient
	fmt.Println("======================================================================")
	fmt.Println("  SCENARIO 3: Attack - Wrong Recipient")
	fmt.Println("======================================================================")
	fmt.Println()

	fmt.Println("ATTACK: Attacker replaces recipient with their own address")
	fmt.Println()

	attackedPayments2 := []t2z.Payment{
		{Address: attackerAddress, Amount: paymentAmount},
	}

	maliciousRequest2, _ := t2z.NewTransactionRequest(attackedPayments2)
	defer maliciousRequest2.Free()

	fmt.Println("User verifies PCZT before signing...")
	err = t2z.VerifyBeforeSigning(proved, maliciousRequest2, []t2z.TransparentOutput{})
	if err != nil {
		fmt.Println("   ATTACK DETECTED! Verification failed:")
		fmt.Printf("   Error: %v\n\n", err)
		fmt.Println("   Transaction NOT signed - funds are SAFE!")
	} else {
		fmt.Println("   DANGER: Verification passed (should not happen!)")
	}
	fmt.Println()

	// SCENARIO 4: Attack - Lower Amount
	fmt.Println("======================================================================")
	fmt.Println("  SCENARIO 4: Attack - Different Payment Amount (Lower)")
	fmt.Println("======================================================================")
	fmt.Println()

	fmt.Println("ATTACK: Attacker claims user only wants to send half the amount")
	lowerAmount := paymentAmount / 2
	fmt.Println()

	attackedPayments3 := []t2z.Payment{
		{Address: victimAddress, Amount: lowerAmount},
	}

	maliciousRequest3, _ := t2z.NewTransactionRequest(attackedPayments3)
	defer maliciousRequest3.Free()

	fmt.Println("User verifies PCZT before signing...")
	err = t2z.VerifyBeforeSigning(proved, maliciousRequest3, []t2z.TransparentOutput{})
	if err != nil {
		fmt.Println("   ATTACK DETECTED! Verification failed:")
		fmt.Printf("   Error: %v\n\n", err)
		fmt.Println("   Transaction NOT signed - funds are SAFE!")
	} else {
		fmt.Println("   DANGER: Verification passed (should not happen!)")
	}
	fmt.Println()

	// Clean up the original PCZT
	proved.Free()

	fmt.Println("KEY TAKEAWAY: Always call VerifyBeforeSigning() before signing!")
	fmt.Println()
	fmt.Println("NOTE: This example did NOT broadcast any transactions.")
	fmt.Println("      UTXOs are still available for other examples.\n")
}
