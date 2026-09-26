Feature: API key management, gateway auth, and rate limits

  Scenario: Admin creates, deactivates, and deletes API keys from the dashboard
    Given I log in to the dashboard as an admin
    When I open the API keys page
    And I create an API key named "dev-key" with RPM 10 and TPM 10000
    Then I see the raw API key shown once
    And the "dev-key" API key appears in the keys table with RPM 10 and TPM 10000
    When I deactivate the "dev-key" API key
    Then the "dev-key" API key is inactive
    When I delete the "dev-key" API key
    Then no API key named "dev-key" appears in the keys table

  Scenario: Gateway rejects requests with missing or invalid API keys
    Given I create an active API key via the admin API named "gateway-key"
    And the admin requires API keys
    When I send a chat completion request with no API key
    Then the gateway returns 401 with "missing API key"
    When I send a chat completion request with an invalid API key
    Then the gateway returns 401 with "invalid API key"
    When I create a second API key named "temp-key" and deactivate it
    And I send a chat completion request with the deactivated "temp-key" API key
    Then the gateway returns 401 with "invalid API key"
    When I send a chat completion request with the valid "gateway-key" API key
    Then the gateway grants the request

  Scenario: Requiring API keys follows the admin setting, not whether keys exist
    Given I create an active API key via the admin API named "optional-key"
    And the admin does not require API keys
    When I send a chat completion request with no API key
    Then the gateway grants the request
    When the admin requires API keys
    And I send a chat completion request with no API key
    Then the gateway returns 401 with "missing API key"
    When I delete every API key via the admin API
    And I send a chat completion request with no API key
    Then the gateway returns 401 with "missing API key"
    When the admin does not require API keys
    And I send a chat completion request with no API key
    Then the gateway grants the request

  Scenario: Admin requires or allows API keys from the dashboard
    Given I log in to the dashboard as an admin
    And I create an active API key via the admin API named "toggle-key"
    And the admin does not require API keys
    When I open the API keys page
    And I turn on requiring API keys
    Then API keys are required for gateway requests
    When I send a chat completion request with no API key
    Then the gateway returns 401 with "missing API key"
    When I turn off requiring API keys
    Then API keys are not required for gateway requests
    When I send a chat completion request with no API key
    Then the gateway grants the request

  Scenario: API keys enforce per-key RPM rate limits on the gateway
    Given I create an API key via the admin API with RPM 2
    When I send 3 chat completion requests with that API key
    Then the gateway rate-limits the 3rd request with 429 "rpm limit exceeded"

  Scenario: A user logging in with an API key is scoped to their usage dashboard
    Given I create an active API key via the admin API named "user-key"
    When I log in to the dashboard using the "user-key" API key
    Then I land on the usage dashboard
    And I do not see the sidebar nav items "Providers, Models, API Keys, Settings, Logs"
    And the dashboard is accessible with the "user-key" API key
