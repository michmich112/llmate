Feature: Dashboard smoke tests

  Scenario: Dashboard pages render without breaking
    Given I am authenticated
    And I create a provider named "Smoke Provider" with base URL "http://127.0.0.1:9000/v1"
    When I visit the route "/providers"
    And I visit the route "/dashboard/models"
    Then no real console errors were logged
    And I take a screenshot named "smoke-pages"

  Scenario: No console errors across dashboard pages
    Given I am authenticated
    When I visit each dashboard page "/, /providers, /logs, /settings, /dashboard/models"
    Then no real console errors were logged
    And I take a screenshot named "smoke-console"
