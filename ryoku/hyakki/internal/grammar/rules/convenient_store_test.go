package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestConvenientStore_Fix711(t *testing.T) {
	linttest.AssertSuggestionResult(t, "7-11 convenient stores is 20 steps away!", NewConvenientStore(), "7-11 convenience stores is 20 steps away!")
}

func TestConvenientStore_FixNearby(t *testing.T) {
	linttest.AssertSuggestionResult(t, "hi, is there any convenient stores nearby?", NewConvenientStore(), "hi, is there any convenience stores nearby?")
}

func TestConvenientStore_FixAround(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Yes they do have convenient stores around.", NewConvenientStore(), "Yes they do have convenience stores around.")
}

func TestConvenientStore_Fix247(t *testing.T) {
	linttest.AssertSuggestionResult(t, "There is also a convenient store just downstairs which is 24/7.", NewConvenientStore(), "There is also a convenience store just downstairs which is 24/7.")
}

func TestConvenientStore_FixCoupons(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Mobile Application for small Convenient stores to keep track of the coupons by scanning the coupon barcode.", NewConvenientStore(), "Mobile Application for small Convenience stores to keep track of the coupons by scanning the coupon barcode.")
}

func TestConvenientStore_FixMalaysia(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Find deals on Kanak Kanak Convenient Store products online with Lazada Malaysia", NewConvenientStore(), "Find deals on Kanak Kanak Convenience Store products online with Lazada Malaysia")
}

func TestConvenientStore_FixSingapore(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Buy Sweet Bean Convenient Store in Singapore,Singapore.", NewConvenientStore(), "Buy Sweet Bean Convenience Store in Singapore,Singapore.")
}

func TestConvenientStore_FixShoppingSnacks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ok, so I was at a convenient store and I noticed this young woman reading through magazines while I was shopping for some snacks.", NewConvenientStore(), "Ok, so I was at a convenience store and I noticed this young woman reading through magazines while I was shopping for some snacks.")
}

func TestConvenientStore_FixShoppingMallRestaurants(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Connected to Mercury Ville shopping mall comprising of restaurants, convenient stores, clinics and banks", NewConvenientStore(), "Connected to Mercury Ville shopping mall comprising of restaurants, convenience stores, clinics and banks")
}

func TestConvenientStore_FixMinimart(t *testing.T) {
	linttest.AssertSuggestionResult(t, "There are many convenient store, 7-11 minimart,Tesco Lotus supermarket,Century Mall, Big C Supermarket,Fresh market . ", NewConvenientStore(), "There are many convenience store, 7-11 minimart,Tesco Lotus supermarket,Century Mall, Big C Supermarket,Fresh market . ")
}

func TestConvenientStore_DontFlagApi(t *testing.T) {
	linttest.AssertNoLints(t, "Convenient \"Store\" API (deprecates ObjectBox and Builder API)", NewConvenientStore())
}

func TestConvenientStore_DontFlagGui(t *testing.T) {
	linttest.AssertNoLints(t, "It can be used for a small convenient store after certain modifications to the GUI.", NewConvenientStore())
}

func TestConvenientStore_DontFlagMethodsDocs(t *testing.T) {
	linttest.AssertNoLints(t, "Add convenient store methods; Improve the docs; Improve some internals.", NewConvenientStore())
}

func TestConvenientStore_DontFlagStored(t *testing.T) {
	linttest.AssertNoLints(t, "So, managing and transferring states becomes easier as all the states are stored in the same convenient store.", NewConvenientStore())
}

func TestConvenientStore_DontFlagValueType(t *testing.T) {
	linttest.AssertNoLints(t, "A safe and convenient store for one value of each type.", NewConvenientStore())
}

func TestConvenientStore_DontFlagRedis(t *testing.T) {
	linttest.AssertNoLints(t, "A Redis server, running on the local client device can serve as a convenient store for messages.", NewConvenientStore())
}

func TestConvenientStore_DongFlagYaml(t *testing.T) {
	linttest.AssertNoLints(t, "Two reasons: it's a more convenient store than yaml, especially for binary data, and it's easier to re-encrypt when gpg-keys change.", NewConvenientStore())
}

func TestConvenientStore_DontFlagMini(t *testing.T) {
	linttest.AssertNoLints(t, "Mini glass bottle collection of convenient store.", NewConvenientStore())
}

func TestConvenientStore_DontFlagFrameDump(t *testing.T) {
	linttest.AssertNoLints(t, "GE frame dump at convenient store.", NewConvenientStore())
}

func TestConvenientStore_DontFlagConvenientStoreMeansHandy(t *testing.T) {
	linttest.AssertNoLints(t, "It is a great feature as customers can choose a convenient store for their order delivery and they know when it can be picked up.", NewConvenientStore())
}

func TestConvenientStore_DontFlag(t *testing.T) {
	linttest.AssertNoLints(t, "It also allows you to create a convenient store that includes any additional structure and the actions and selectors to be used to manage the entities.", NewConvenientStore())
}
