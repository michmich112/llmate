Feature: Provider management

  Scenario: Provider rename end-to-end
    Given I am authenticated
    And I create a provider named "E2E Provider" with base URL "http://127.0.0.1:8000/v1"
    When I visit the route "/providers"
    Then I see the provider "E2E Provider" in the table
    And I take a screenshot named "06-providers-list"
    When I open the edit page for "E2E Provider"
    Then I see the name input on the provider detail page
    And I take a screenshot named "07-provider-detail"
    When I rename the provider to "Renamed Provider"
    Then the provider heading shows "Renamed Provider"
    And the provider name persisted is "Renamed Provider"
    And I take a screenshot named "08-provider-renamed"
