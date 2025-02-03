---
sidebar_position: 5
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# OpenTelemetry for Python

Instrumenting a Python application with OpenTelemetry can provide valuable insights into the application's performance and behavior. 
OpenTelemetry is an open-source observability framework that enables the collection and exporting of telemetry data. 
This document covers the steps required to instrument a Python application with OpenTelemetry.

## Auto-instrumentation

OpenTelemetry provides a Python agent that automatically detects and instruments the most popular application servers, clients, and frameworks. 
This means we don't even need to change the code of our app to instrument it.

Let's start with a simple web application.

<Tabs queryString="framework">
  <TabItem value="django" label="Django" default>

A simple Django view handler:

```python
def hello(request, name):
  return HttpResponse("Hello, {}!".format(name))
```

Install OpenTelemetry dependencies:

```bash
pip install opentelemetry-distro opentelemetry-exporter-otlp
opentelemetry-bootstrap -a install
```

Then, run the application with the instrumentation:

```bash
export DJANGO_SETTINGS_MODULE=otel_django.settings \
  OTEL_RESOURCE_ATTRIBUTES="service.name=django-app" \
  OTEL_EXPORTER_OTLP_TRACES_ENDPOINT="http://coroot.coroot:8080/v1/traces" \
  OTEL_EXPORTER_OTLP_TRACES_PROTOCOL="http/protobuf" \
&& opentelemetry-instrument --traces_exporter otlp --metrics_exporter none ./manage.py runserver --noreload 8000
```

As a result, our app reports traces to the configured OpenTelemetry collector:
<img alt="Python django trace" src="/img/docs/python_django_trace.png" class="card w-1200"/>

Span attributes:
<img alt="Python django server span attributes" src="/img/docs/python_django_server_span_attributes.png" class="card w-1200"/>

  </TabItem>
  <TabItem value="fastapi" label="FastAPI">

A simple FastAPI app:

```python
from fastapi import FastAPI

app = FastAPI()

@app.get("/hello/{name}")
async def say_hello(name: str):
  return {"message": f"Hello, {name}!"}
```

Install OpenTelemetry dependencies:

```bash
pip install opentelemetry-distro opentelemetry-exporter-otlp
opentelemetry-bootstrap -a install
```

Then, run the application with the instrumentation:

```bash
export DJANGO_SETTINGS_MODULE=otel_django.settings \
  OTEL_RESOURCE_ATTRIBUTES="service.name=django-app" \
  OTEL_EXPORTER_OTLP_TRACES_ENDPOINT="http://coroot.coroot:8080/v1/traces" \
  OTEL_EXPORTER_OTLP_TRACES_PROTOCOL="http/protobuf" \
&& opentelemetry-instrument --traces_exporter otlp --metrics_exporter none uvicorn main:app
```

As a result, our app reports traces to the configured OpenTelemetry collector:
<img alt="Python FastAPI trace" src="/img/docs/python_fastapi_trace.png" class="card w-1200"/>

Span attributes:
<img alt="Python FastAPI server span attributes" src="/img/docs/python_fastapi_server_span_attributes.png" class="card w-1200"/>

  </TabItem>
  <TabItem value="flask" label="Flask">

A simple Flask app:

```python
from flask import Flask

app = Flask(__name__)

@app.route('/hello/<name>')
def hello(name):
  return 'Hello, {}!'.format(name)

if __name__ == '__main__':
  app.run()
```

Install OpenTelemetry dependencies:

```bash
pip install opentelemetry-distro opentelemetry-exporter-otlp
opentelemetry-bootstrap -a install
```

Then, run the application with the instrumentation:

```bash
export DJANGO_SETTINGS_MODULE=otel_django.settings \
  OTEL_RESOURCE_ATTRIBUTES="service.name=django-app" \
  OTEL_EXPORTER_OTLP_TRACES_ENDPOINT="http://coroot.coroot:8080/v1/traces" \
  OTEL_EXPORTER_OTLP_TRACES_PROTOCOL="http/protobuf" \
&& opentelemetry-instrument --traces_exporter otlp --metrics_exporter none flask run
```

As a result, our app reports traces to the configured OpenTelemetry collector:
<img alt="Python Flask trace" src="/img/docs/python_flask_trace.png" class="card w-1200"/>

Span attributes:
<img alt="Python Flask server span attributes" src="/img/docs/python_flask_server_span_attributes.png" class="card w-1200"/>
  </TabItem>
</Tabs>

## Exceptions

Now, let's explore what happens when our app raises an exception.

```python 
def hello(request, name):
  raise Exception("Failure injection")
  return HttpResponse("Hello, {}!".format(name))
``` 

<img alt="Python Error trace" src="/img/docs/python_error_trace.png" class="card w-1200"/>

The server span captures the exception and includes the corresponding traceback for better analysis and debugging:

<img alt="Python Error Span Attributes" src="/img/docs/python_error_span_attributes.png" class="card w-1200"/>

As you can see, we have easily identified the reason why this particular request resulted in an error.

## Database calls

Now, let's add a database call to our app:

```python 
from django.http import HttpResponse
from hello_app.models import Person

def hello(request, id):
  p = Person.objects.get(id=id)
  return HttpResponse("Hello, {}!".format(p.name))
```

Once again, there is no need to add any additional instrumentation as the OTel Python agent automatically captures every database call.
<img alt="Python Trace with DB calls" src="/img/docs/python_trace_with_db_call.png" class="card w-1200"/>

Span attributes:
<img alt="Python DB call span attributes" src="/img/docs/python_db_call_span_attributes.png" class="card w-1200"/>

## HTTP calls and context propagation

Next, instead of retrieving a user from the database, let's make an HTTP call to a service and retrieve a JSON response containing the desired information.

```python
import requests
from django.http import HttpResponse

def hello(request, id):
  r = requests.get('http://127.0.0.1:8082/user/{}'.format(id))
  name = r.json()['name']
  return HttpResponse("Hello, {}!".format(name))
```

<img alt="Python trace with HTTP calls" src="/img/docs/python_trace_with_http_call.png" class="card w-1200"/>

Client span attributes:
<img alt="Python HTTP call span attributes" src="/img/docs/python_http_call_span_attributes.png" class="card w-1200"/>


As you can see, the resulting trace includes a span reported by the user service. Both services are instrumented with OpenTelemetry. 
But how is the context of the current trace propagated between them? To gain a better understanding of context propagation, 
let's examine the request sent to the user service:

```GET /user/2 HTTP/1.1
Host: 127.0.0.1
Connection: keep-alive
User-agent: python-requests/2.30.0
Accept: */*
Traceparent: 00-7d4ab2226954f6f712b8be0c067b21f6-b327da7466332edf-01
```

OpenTelemetry adds the `Traceparent` HTTP header on the client side, and dependency services read this header to propagate 
the trace context. It has the following format:

`Version-TraceID-ParentSpanID-TraceFlags`

In our case, `7d4ab2226954f6f712b8be0c067b21f6` is the TraceID, and `b327da7466332edf` is the ParentSpanId.

## Adding custom attributes and events to spans

If needed, you can retrieve the current span and set custom attributes or add events.

```python
import requests
from django.http import HttpResponse
from opentelemetry import trace

def hello(request, id):
  span = trace.get_current_span()

  # set an attribute
  span.set_attribute('user.id', id)
  
  r = requests.get('http://127.0.0.1:8082/user/{}'.format(id))
  
  # add an event
  span.add_event('the user profile has been loaded from the user service')
  
  name = r.json()['name']
  return HttpResponse("Hello, {}!".format(name))
```

The resulting span:

<img alt="Python custom span attributes and events" src="/img/docs/python_custom_attributes_and_events.png" class="card w-1200"/>


