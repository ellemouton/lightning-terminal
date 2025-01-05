include make/compile_flags.mk

ITEST_FLAGS =
TEST_FLAGS =
DEV_TAGS = dev

# Define the integration test.run filter if the icase argument was provided.
ifneq ($(icase),)
ITEST_FLAGS += -test.run="TestLightningTerminal/$(icase)"
endif

# If a specific unit test case is being target, construct test.run filter.
ifneq ($(case),)
TEST_FLAGS += -test.run=$(case)
UNIT_TARGETED = yes
endif

# If specific package is being unit tested, construct the full name of the
# subpackage.
ifneq ($(pkg),)
UNITPKG := $(PKG)/$(pkg)
COVER_PKG := $(PKG)/$(pkg)
UNIT_TARGETED = yes
GOLIST = echo '$(PKG)/$(pkg)'
endif

# Add any additional tags that are passed in to make.
ifneq ($(tags),)
DEV_TAGS += ${tags}
endif

# UNIT_TARGETED is undefined iff a specific package and/or unit test case is
# not being targeted.
UNIT_TARGETED ?= no

# If a specific package/test case was requested, run the unit test for the
# targeted case. Otherwise, default to running all tests.
ifeq ($(UNIT_TARGETED), yes)
UNIT := $(GOTEST) -tags="$(DEV_TAGS) $(COMPILE_TAGS)" $(TEST_FLAGS) $(UNITPKG)
UNIT_RACE := $(GOTEST) -tags="$(DEV_TAGS) $(COMPILE_TAGS)" $(TEST_FLAGS) -race $(UNITPKG)
endif

ifeq ($(UNIT_TARGETED), no)
UNIT := $(GOLIST) | $(XARGS) env $(GOTEST) -tags="$(DEV_TAGS) $(COMPILE_TAGS)" $(TEST_FLAGS)
UNIT_RACE := $(UNIT) -race
endif