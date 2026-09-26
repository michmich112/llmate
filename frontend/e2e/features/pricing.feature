Feature: Pricing and cost analytics

  Scenario: Provider cost rates flow into the lifetime cost card
    Given I am authenticated
    And I create a provider named "Pricing E2E" with base URL "http://127.0.0.1:9005/v1"
    And I register model "llama3" on that provider
    And I set cost rates input 2.0 output 3.0 cache-read 0.5 on that model
    And I seed request logs with 1000 prompt 500 completion 200 cached tokens
    When I visit the route "/"
    And I switch to Lifetime mode
    Then I see the text "Est. Total Cost"
    And the lifetime cost card shows a non-zero dollar amount
    And I take a screenshot named "pricing-lifetime"
