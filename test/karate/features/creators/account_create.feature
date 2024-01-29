@Ignore
Feature: Account create helper
  Background:
    * url baseUrl.api
    * configure logPrettyRequest = true
    * configure logPrettyResponse = true
    * def sleep = function(millis){ java.lang.Thread.sleep(millis) }

  Scenario: Register account to use on other features
    Given path "account"
    And request __arg
    When method post
    Then status 201
    And match response == read("account/response-register-account.json")
    * sleep(1000)
