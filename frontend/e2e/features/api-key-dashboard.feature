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

  Scenario: API-key user visiting an admin-only page is redirected to the dashboard
    Given I create an active API key via the admin API named "user-key"
    When I log in to the dashboard using the "user-key" API key
    And I visit the admin-only route "/providers"
    Then I am redirected to the dashboard

  Scenario: Key-scoped stats omit provider fields
    Given I create an active API key via the admin API named "user-key"
    When I request my dashboard stats with the "user-key" API key
    Then the dashboard stats omit provider fields
    And the dashboard stats omit the API key breakdown

  Scenario: API-key user does not see provider information
    Given I create an active API key via the admin API named "user-key"
    When I log in to the dashboard using the "user-key" API key
    Then I land on the usage dashboard
    And I do not see provider information
    And I do not see the "Requests by API Key" table

  Scenario: Admin opening /usage lands on the dashboard
    Given I log in to the dashboard as an admin
    When I visit the admin-only route "/usage"
    Then I am redirected to the dashboard
    And I see provider information

  Scenario: Bookmarked /my-usage returns to the dashboard when signed in
    Given I log in to the dashboard as an admin
    When I visit the admin-only route "/my-usage"
    Then I am redirected to the dashboard

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

  Scenario: Admin sees requests broken down by API key
    Given I log in to the dashboard as an admin
    And I create an active API key via the admin API named "dash-alice"
    And I seed request logs for the API key named "dash-alice" with 5 requests over the last 6 hours
    And I create an active API key via the admin API named "dash-bob"
    And I seed request logs for the API key named "dash-bob" with 3 requests over the last 6 hours
    When I open the dashboard
    And I select the "Lifetime" dashboard range
    Then I see the "Requests by API Key" table
    And the API key breakdown includes "dash-alice" with 5 requests
    And the API key breakdown includes "dash-bob" with 3 requests

  Scenario: Admin sees usage across all API keys
    Given I create an active API key via the admin API named "alice"
    And I seed request logs for the API key named "alice" with 5 requests over the last 6 hours
    And I create an active API key via the admin API named "bob"
    And I seed request logs for the API key named "bob" with 3 requests over the last 6 hours
    When I query the admin usage endpoint
    Then the admin usage endpoint includes the "alice" API key with 5 requests
    And the admin usage endpoint includes the "bob" API key with 3 requests
