Feature: Authentication and access control

  Scenario: Unauthenticated access to dashboard routes redirects to /login
    Given I am not authenticated
    When I visit the protected routes "/providers, /logs, /settings, /dashboard/models"
    Then I am redirected to /login

  Scenario: Login with an invalid access key shows an error and stays on /login
    Given I am on the login page
    When I fill the access key input with "wrong-key"
    And I click the "Sign in" button
    Then I see the text "Invalid access key"
    And I remain on /login

  Scenario: Login with a valid access key lands on the dashboard
    Given I am on the login page
    When I fill the access key input with "e2e-key"
    And I click the "Sign in" button
    Then I land on the dashboard
    And I take a screenshot named "auth-login-success"
