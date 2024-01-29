@Ignore
Feature: Tags create helper
  Background:
    * url baseUrl.api
    * configure logPrettyRequest = true
    * configure logPrettyResponse = true
    * def sleep = function(millis){ java.lang.Thread.sleep(millis) }

  Scenario: Add tags to use on other features
    Given path "/tag"
    And request
    """
    {
      "tagCategoryId": "#(tagCategoryId)",
      "tagName": "#(tag)",
      "tagNotes": ""
    }
    """
    When method post
    Then status 201
    And match response == read("tag/response-add-tag-to-existing-category.json")
    * sleep(1000)