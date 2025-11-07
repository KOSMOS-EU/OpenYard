# OpenYard Cloud API 1.1 - Documentation

Streamlined API documentation for OpenYard Cloud integration with OpenCloud and Reva platforms.

## Overview

**Version:** 1.1.0
**Title:** OpenYard Cloud API

OpenYard Cloud API for integration with OpenCloud and Reva platforms. Provides CS3-compatible document management, WebDAV support, and OCS Share API compatibility.

## Servers

- **Production OpenCloud server**: `https://cloud.brandis.eu`
- **Local Reva development server**: `https://localhost:9200`

## Authentication

OpenYard Cloud API supports multiple authentication methods:

### Session Authentication
```http
GET /api/documents/GetDocument?SessionID=your-session-id&ObjectId=123
```

### Bearer Token (Recommended for Reva)
```http
GET /api/documents/GetDocument?ObjectId=123
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### Basic Authentication (WebDAV Compatible)
```http
GET /api/documents/GetDocument?ObjectId=123
Authorization: Basic dXNlcm5hbWU6cGFzc3dvcmQ=
```

## API Endpoints

### Documents

#### POST `/api/advancedDocuments/BinDocuments`
**Operation ID:** `BinDocuments`

**Summary:** Move to Recycle Bin

**Description:** Move documents to trash/recycle bin

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
POST /api/advancedDocuments/BinDocuments?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### POST `/api/advancedDocuments/GetDeletedFiles`
**Operation ID:** `GetDeletedFiles`

**Summary:** List Deleted Documents

**Description:** List deleted files from trash

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
POST /api/advancedDocuments/GetDeletedFiles?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### POST `/api/advancedDocuments/GetDocument`
**Operation ID:** `GetDocument`

**Summary:** Get Document Details

**Description:** Retrieve document metadata and content information

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
POST /api/advancedDocuments/GetDocument?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### GET `/api/advancedDocuments/GetDocumentVersions`
**Operation ID:** `GetDocumentVersions`

**Summary:** Get Version History

**Description:** Get document version history

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
GET /api/advancedDocuments/GetDocumentVersions?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### POST `/api/advancedDocuments/GetFile`
**Operation ID:** `GetFile`

**Summary:** Download Document

**Description:** Download document binary content

**Example Request:**
```http
POST /api/advancedDocuments/GetFile
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### GET `/api/advancedDocuments/GetPreviewFile`
**Operation ID:** `GetPreviewFile`

**Summary:** Generate Document Preview

**Description:** Generate document preview or thumbnail

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
GET /api/advancedDocuments/GetPreviewFile?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### GET `/api/advancedDocuments/IsDocument`
**Operation ID:** `IsDocument`

**Summary:** Verify Document Type

**Description:** Check if object is a document

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |
| ObjectId | string | ✅ | Unique identifier of the target object (document or folder) |

**Example Request:**
```http
GET /api/advancedDocuments/IsDocument?SessionID=abc123-def456-ghi789&ObjectId=doc_12345
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### POST `/api/advancedDocuments/SetDocument`
**Operation ID:** `SetDocument`

**Summary:** Create/Update Document

**Description:** Create or update documents with content and metadata

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Request Body:**

File content to upload

**Content Types:**

- `multipart/form-data`
- `application/octet-stream`

**Example Request:**
```http
POST /api/advancedDocuments/SetDocument?SessionID=abc123-def456-ghi789
Content-Type: application/json

{
  "Name": "contract.pdf",
  "FolderId": "folder_123",
  "Content": "base64_encoded_content"
}
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### POST `/api/advancedDocuments/SetFile`
**Operation ID:** `SetFile`

**Summary:** Upload Document Content

**Description:** Upload document binary content

**Request Body:**

File content to upload

**Content Types:**

- `multipart/form-data`
- `application/octet-stream`

**Example Request:**
```http
POST /api/advancedDocuments/SetFile
Content-Type: application/json

// File upload as multipart/form-data
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

### Folders

#### POST `/api/advancedFolders/DeleteFolders`
**Operation ID:** `DeleteFolders`

**Summary:** Delete Folders

**Description:** Delete folders and contents

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
POST /api/advancedFolders/DeleteFolders?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### POST `/api/advancedFolders/GetFolder`
**Operation ID:** `GetFolder`

**Summary:** Get Folder Information

**Description:** Retrieve folder information and contents

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
POST /api/advancedFolders/GetFolder?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### POST `/api/advancedFolders/GetFolderByFolderpath`
**Operation ID:** `GetFolderByFolderpath`

**Summary:** Find Folder by Path

**Description:** Find folder by hierarchical path

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| FolderPath | string | ❌ | Complete hierarchical path to the folder location |
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
POST /api/advancedFolders/GetFolderByFolderpath?FolderPath=/Documents/Projects&SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### GET `/api/advancedFolders/IsFolder`
**Operation ID:** `IsFolder`

**Summary:** Verify Folder Type

**Description:** Check if object is a folder

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |
| ObjectId | string | ✅ | Unique identifier of the target object (document or folder) |

**Example Request:**
```http
GET /api/advancedFolders/IsFolder?SessionID=abc123-def456-ghi789&ObjectId=doc_12345
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### POST `/api/advancedFolders/SetFolder`
**Operation ID:** `SetFolder`

**Summary:** Create/Update Folder

**Description:** Create or update folder structure

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
POST /api/advancedFolders/SetFolder?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

### Search

#### POST `/api/advancedCommon/Search`
**Operation ID:** `Search`

**Summary:** Advanced Search

**Description:** Advanced search across content and metadata

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
POST /api/advancedCommon/Search?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### GET `/api/advancedDocuments/GetMetaData`
**Operation ID:** `GetMetaData`

**Summary:** Get Document Metadata

**Description:** Get comprehensive object metadata

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
GET /api/advancedDocuments/GetMetaData?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### GET `/api/basicCommon/GetIndexData`
**Operation ID:** `GetIndexData`

**Summary:** Get Index Information

**Description:** Retrieve object indexing information

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
GET /api/basicCommon/GetIndexData?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### GET `/api/basicCommon/GetMetaData`
**Operation ID:** `GetMetaData`

**Summary:** Get Document Metadata

**Description:** Get comprehensive object metadata

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
GET /api/basicCommon/GetMetaData?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### POST `/api/basicCommon/SearchForTitle`
**Operation ID:** `SearchForTitle`

**Summary:** Search by Title

**Description:** Search by document/folder titles

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
POST /api/basicCommon/SearchForTitle?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

### Sharing

#### GET `/api/advancedFolders/GetFolderRights`
**Operation ID:** `GetFolderRights`

**Summary:** Get Folder Permissions

**Description:** Get folder access permissions

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
GET /api/advancedFolders/GetFolderRights?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### POST `/api/advancedFolders/SetFolderRights`
**Operation ID:** `SetFolderRights`

**Summary:** Set Folder Permissions

**Description:** Set folder access permissions

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
POST /api/advancedFolders/SetFolderRights?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### GET `/api/basicCommon/GetExistingPermalinksByObjId`
**Operation ID:** `GetPermaLinkByObjId`

**Summary:** Generate Permalink

**Description:** Generate permanent links

**Example Request:**
```http
GET /api/basicCommon/GetExistingPermalinksByObjId
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### GET `/api/basicCommon/DeletePermalink`
**Operation ID:** `DeletePermalink`

**Summary:** Remove Permalink

**Description:** Remove permalinks

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
GET /api/basicCommon/DeletePermalink?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### GET `/api/basicCommon/GetPreviewFileByPermalink`
**Operation ID:** `GetPreviewFileByPermalink`

**Summary:** Preview via Permalink

**Description:** Preview via permalink

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
GET /api/basicCommon/GetPreviewFileByPermalink?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### GET `/api/basicCommon/GetObjectIdByPermalink`
**Operation ID:** `GetObjectIdByPermalink`

**Summary:** Resolve Permalink

**Description:** Resolve permalink to object

**Example Request:**
```http
GET /api/basicCommon/GetObjectIdByPermalink
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

### Authentication

#### GET `/api/advancedUsers/GetSessionBySessionID`
**Operation ID:** `GetSessionBySessionID`

**Summary:** Get Session Info

**Description:** Retrieve session details

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
GET /api/advancedUsers/GetSessionBySessionID?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### POST `/api/advancedUsers/GetUserInfo`
**Operation ID:** `GetUserInfo`

**Summary:** Get User Details

**Description:** Get user account information

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
POST /api/advancedUsers/GetUserInfo?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### GET `/api/advancedUsers/Authenticate`
**Operation ID:** `Authenticate`

**Summary:** Verify Authentication

**Description:** Verify user credentials

**Example Request:**
```http
GET /api/advancedUsers/Authenticate
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### POST `/api/advancedUsers/Login`
**Operation ID:** `Login`

**Summary:** User Login

**Description:** User authentication and session creation

**Example Request:**
```http
POST /api/advancedUsers/Login
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### GET `/api/advancedUsers/Logout`
**Operation ID:** `Logout`

**Summary:** User Logout

**Description:** Session termination

**Example Request:**
```http
GET /api/advancedUsers/Logout
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

### Operations

#### POST `/api/advancedCommon/CopyObjects`
**Operation ID:** `CopyObjects`

**Summary:** Copy Objects

**Description:** Copy files and folders

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
POST /api/advancedCommon/CopyObjects?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### POST `/api/advancedCommon/MoveObjects`
**Operation ID:** `MoveObjects`

**Summary:** Move Objects

**Description:** Move files and folders

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
POST /api/advancedCommon/MoveObjects?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### POST `/api/advancedCommon/RecoverObjects`
**Operation ID:** `RecoverObjects`

**Summary:** Restore Deleted Objects

**Description:** Restore deleted objects from trash

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
POST /api/advancedCommon/RecoverObjects?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

### System

#### GET `/api/advancedCommon/ExistObjectId`
**Operation ID:** `ExistObjectId`

**Summary:** Check Object Existence

**Description:** Check object existence by ID

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |
| ObjectId | string | ✅ | Unique identifier of the target object (document or folder) |

**Example Request:**
```http
GET /api/advancedCommon/ExistObjectId?SessionID=abc123-def456-ghi789&ObjectId=doc_12345
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### GET `/api/advancedCommon/ExistObjectPath`
**Operation ID:** `ExistObjectPath`

**Summary:** Validate Object Path

**Description:** Check object existence by path

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |
| ObjectPath | string | ❌ | Complete hierarchical path to the object within the folder structure |

**Example Request:**
```http
GET /api/advancedCommon/ExistObjectPath?SessionID=abc123-def456-ghi789&ObjectPath=/Documents/Contracts/contract.pdf
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### GET `/api/advancedGeneral/GetServerSettings`
**Operation ID:** `GetServerSettings`

**Summary:** Get System Settings

**Description:** Get server configuration

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
GET /api/advancedGeneral/GetServerSettings?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### GET `/api/advancedGeneral/IsListening`
**Operation ID:** `IsListening`

**Summary:** Check System Status

**Description:** Check API server status

**Example Request:**
```http
GET /api/advancedGeneral/IsListening
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### GET `/api/basicCommon/GetFolderSubElementsByMemberId`
**Operation ID:** `GetFolderSubElementsByMemberId`

**Summary:** Get Folder Contents by Member

**Description:** Get folder contents by member

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |
| ObjectId | string | ✅ | Unique identifier of the target object (document or folder) |

**Example Request:**
```http
GET /api/basicCommon/GetFolderSubElementsByMemberId?SessionID=abc123-def456-ghi789&ObjectId=doc_12345
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

#### GET `/api/basicConfig/GetOpenyardDMSUserSettings`
**Operation ID:** `GetOpenyardDMSUserSettings`

**Summary:** Get User Preferences

**Description:** Get user preferences

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SessionID | string | ❌ | Unique session identifier for user authentication and request authorization |

**Example Request:**
```http
GET /api/basicConfig/GetOpenyardDMSUserSettings?SessionID=abc123-def456-ghi789
```

**Responses:**

- **200**: Successful operation
- **400**: Bad request
- **401**: Authentication required
- **403**: Access forbidden
- **404**: Resource not found
- **500**: Internal server error

---

## Integration Examples

### WebDAV Mapping

OpenYard Cloud API operations can be mapped to standard WebDAV operations:

| WebDAV | OpenYard API | Description |
|--------|--------------|-------------|
| `PROPFIND /folder/` | `GET /api/folders/GetFolder` | Get folder properties |
| `GET /file.pdf` | `GET /api/documents/GetFile` | Download file |
| `PUT /file.pdf` | `POST /api/documents/SetFile` | Upload file |
| `DELETE /file.pdf` | `POST /api/documents/BinDocuments` | Delete file |
| `MOVE /old /new` | `POST /api/operations/MoveObjects` | Move file/folder |
| `COPY /src /dst` | `POST /api/operations/CopyObjects` | Copy file/folder |
| `MKCOL /newfolder/` | `POST /api/folders/SetFolder` | Create folder |

### JavaScript SDK Example

```javascript
// OpenYard Cloud API Client
class OpenYardClient {
  constructor(baseUrl, token) {
    this.baseUrl = baseUrl;
    this.token = token;
  }

  async request(method, endpoint, params = {}, body = null) {
    const url = new URL(endpoint, this.baseUrl);
    Object.keys(params).forEach(key => {
      url.searchParams.append(key, params[key]);
    });

    const headers = {
      'Authorization': `Bearer ${this.token}`,
      'Content-Type': 'application/json'
    };

    const response = await fetch(url, {
      method,
      headers,
      body: body ? JSON.stringify(body) : null
    });

    return response.json();
  }

  // Document operations
  async getDocument(objectId) {
    return this.request('GET', '/api/documents/GetDocument', { ObjectId: objectId });
  }

  async downloadFile(objectId) {
    return this.request('GET', '/api/documents/GetFile', { ObjectId: objectId });
  }

  async uploadFile(folderId, file) {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('FolderId', folderId);

    return fetch(`${this.baseUrl}/api/documents/SetFile`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${this.token}` },
      body: formData
    });
  }

  // Folder operations
  async getFolder(folderId) {
    return this.request('GET', '/api/folders/GetFolder', { FolderId: folderId });
  }

  async createFolder(parentId, name) {
    return this.request('POST', '/api/folders/SetFolder', {}, {
      ParentId: parentId,
      Name: name
    });
  }

  // Search operations
  async search(query) {
    return this.request('POST', '/api/search/Search', {}, {
      SearchTerm: query
    });
  }
}

// Usage example
const client = new OpenYardClient('https://cloud.brandis.eu', 'your-jwt-token');

// Get folder contents
const folder = await client.getFolder('folder-123');
console.log('Folder contents:', folder);

// Search for documents
const results = await client.search('contract');
console.log('Search results:', results);
```

### Python SDK Example

```python
import requests
from typing import Optional, Dict, Any

class OpenYardClient:
    def __init__(self, base_url: str, token: str):
        self.base_url = base_url.rstrip('/')
        self.token = token
        self.session = requests.Session()
        self.session.headers.update({
            'Authorization': f'Bearer {token}',
            'Content-Type': 'application/json'
        })

    def request(self, method: str, endpoint: str, params: Optional[Dict] = None, json: Optional[Dict] = None) -> Dict[str, Any]:
        url = f'{self.base_url}{endpoint}'
        response = self.session.request(method, url, params=params, json=json)
        response.raise_for_status()
        return response.json()

    # Document operations
    def get_document(self, object_id: str) -> Dict[str, Any]:
        return self.request('GET', '/api/documents/GetDocument', {'ObjectId': object_id})

    def download_file(self, object_id: str) -> bytes:
        response = self.session.get(f'{self.base_url}/api/documents/GetFile', params={'ObjectId': object_id})
        response.raise_for_status()
        return response.content

    def upload_file(self, folder_id: str, file_path: str) -> Dict[str, Any]:
        with open(file_path, 'rb') as f:
            files = {'file': f}
            data = {'FolderId': folder_id}
            response = self.session.post(f'{self.base_url}/api/documents/SetFile', files=files, data=data)
            response.raise_for_status()
            return response.json()

    # Folder operations
    def get_folder(self, folder_id: str) -> Dict[str, Any]:
        return self.request('GET', '/api/folders/GetFolder', {'FolderId': folder_id})

    def create_folder(self, parent_id: str, name: str) -> Dict[str, Any]:
        return self.request('POST', '/api/folders/SetFolder', json={'ParentId': parent_id, 'Name': name})

    # Search operations
    def search(self, query: str) -> Dict[str, Any]:
        return self.request('POST', '/api/search/Search', json={'SearchTerm': query})

# Usage example
client = OpenYardClient('https://cloud.brandis.eu', 'your-jwt-token')

# Get document information
doc = client.get_document('doc-123')
print(f'Document: {doc}')

# Upload a file
result = client.upload_file('folder-456', '/path/to/file.pdf')
print(f'Upload result: {result}')
```

## Error Handling

All endpoints return standard HTTP status codes and JSON error responses:

### Error Response Format
```json
{
  "error": "Authentication required",
  "code": "AUTH_REQUIRED"
}
```

### Common Error Codes

| HTTP Code | Error Code | Description |
|-----------|------------|-------------|
| 400 | `INVALID_REQUEST` | Malformed request parameters |
| 401 | `AUTH_REQUIRED` | Authentication credentials missing or invalid |
| 403 | `ACCESS_DENIED` | Insufficient permissions for operation |
| 404 | `NOT_FOUND` | Requested resource does not exist |
| 409 | `CONFLICT` | Resource conflict (e.g., duplicate name) |
| 413 | `FILE_TOO_LARGE` | Uploaded file exceeds size limit |
| 415 | `UNSUPPORTED_TYPE` | File type not supported |
| 500 | `INTERNAL_ERROR` | Server-side error occurred |

## Rate Limiting

API requests are subject to rate limiting:

- **Standard users**: 1000 requests/hour
- **Premium users**: 5000 requests/hour
- **Enterprise**: Custom limits

Rate limit headers are included in responses:

```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1640995200
```

## Changelog

### Version 1.1.0 (Current)

**Added:**
- CS3 API compatibility
- JWT Bearer token authentication
- WebDAV protocol mapping
- OCS Share API compatibility
- Multi-authentication support

**Changed:**
- Reduced API surface (39 operations vs 126 in v1.0)
- Simplified error responses
- Streamlined permission model

**Removed:**
- Enterprise-only workflow features
- XDOMEA import/export (replaced by CS3)
- Advanced configuration management
- Legacy authentication methods

---

*OpenYard Cloud API 1.1 - Optimized for modern cloud document management*