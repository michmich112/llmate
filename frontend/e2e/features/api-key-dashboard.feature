Feature: API-key user dashboard and access control

  Scenario: API-key user sees their personal usage dashboard
    Given I create an active API key via the admin API named "user-key"
    When I log in to the dashboard using the "user-key" API key
    Then I land on the usage dashboard
    And I see the "Requests by Model" table

  Scenario: API-key user does not see admin-only navigation tabs
    Given I create an active API key via the admin API named "user-key"
    When I log in to the dashboard using the "user-key" API key
    Then I do not see the sidebar nav items "Providers, Models, API Keys, Settings, Logs"

  Scenario: API-key user visiting an admin-only page is redirected to /usage
    Given I create an active API key via the admin API named "user-key"
    When I log in to the dashboard using the "user-key" API key
    And I visit the admin-only route "/providers"
    Then I am redirected to /usage

  Scenario: Admin API endpoints reject an API key with 401
    Given I create an active API key via the admin API named "user-key"
    When I call the admin API endpoints "/admin/providers, /admin/keys, /admin/logs" with the "user-key" API key
    Then each admin endpoint returns 401 with "unauthorized"

  Scenario: API key sees their own request timeseries populate
    Given I create an active API key via the admin API named "usage-key"
    And I seed request logs for the API key named "usage-key" with 5 requests over the last 6 hours
    When I log in to the dashboard using the "usage-key" API key
    Then I land on the usage dashboard
    And I see the Requests metric shows 5
    And the requests chart is rendered

  Scenario: API key users see only their own usage
    Given I create an active API key via the admin API named "alice"
    And I seed request logs for the API key named "alice" with 5 requests over the last 6 hours
    And I create an active API key via the admin API named "bob"
    And I seed request logs for the API key named "bob" with 3 requests over the last 6 hours
    When I log in to the dashboard using the "alice" API key
    Then I land on the usage dashboard
    And I see the Requests metric shows 5
    When I log in to the dashboard using the "bob" API key
    Then I land on the usage dashboard
    And I see the Requests metric shows 3

  Scenario: Admin sees usage across all API keys
    Given I create an active API key via the admin API named "alice"
    And I seed request logs for the API key named "alice" with 5 requests over the last 6 hours
    And I create an active API key via the admin API named "bob"
    And I seed request logs for the API key named "bob" with 3 requests over the last 6 hours
    When I query the admin usage endpoint
    Then the admin usage endpoint includes the "alice" API key with 5 requests
    And the admin usage endpoint includes the "bob" API key with 3 requests
