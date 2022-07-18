package functions;

import java.net.URI;
import java.util.HashMap;
import java.util.Map;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.context.SpringBootTest.WebEnvironment;
import org.springframework.boot.test.web.client.TestRestTemplate;
import org.springframework.http.RequestEntity;
import org.springframework.http.ResponseEntity;

import static org.hamcrest.MatcherAssert.assertThat;
import static org.hamcrest.Matchers.*;

@SpringBootTest(classes = CloudFunctionApplication.class,
  webEnvironment = WebEnvironment.RANDOM_PORT)
public class ReconcileFunctionTest {

  @Autowired
  private TestRestTemplate rest;

  @Test
  public void testReconcile() throws Exception {
    Map<String, Object> payload = new HashMap();
    Map<String, Object> parent = new HashMap();
    parent.put("apiVersion", "metacontroller.github.com/v1");
    parent.put("Kind", "Internet");
    Map<String, Object> metadata = new HashMap();
    Map<String, Object> spec = new HashMap();
    spec.put("production-test-enabled", true);
    parent.put("metadata", metadata);
    parent.put("spec", spec);
    payload.put("parent", parent);
    
    ResponseEntity<Map> response = this.rest.exchange(
      RequestEntity.post(new URI("/reconcile"))
                   .body(payload), Map.class);
    assertThat(response.getStatusCode()
                       .value(), equalTo(200));
    assertThat(response.getBody(), instanceOf(Map.class));
    assertThat(((Map<String, Object>)response.getBody()).get("children"), notNullValue() );
    assertThat(((Map<String, Object>)response.getBody()).get("status"), notNullValue() );
  }
}
