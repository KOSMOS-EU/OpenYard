# OpenYard Cloud API 1.1 Extended - Complete Documentation

## 🚀 Overview

**OpenYard Cloud API 1.1 Extended** provides maximum compatibility with legacy systems while maintaining cloud-native standards. With **76 operations** (compared to 39 in Core), it offers comprehensive mapping layers for CS3, WebDAV, and OCS protocols.

### 📊 API Coverage

| Version | Operations | Coverage | Use Case |
|---------|------------|----------|----------|
| **OpenYard 1.0** | 126 | 100% | Enterprise DMS |
| **OpenYard 1.1 Core** | 39 | 31% | Cloud-native optimized |
| **🎯 OpenYard 1.1 Extended** | **76** | **60%** | **Maximum compatibility** |

---

## 🔧 Authentication

OpenYard 1.1 Extended supports multiple authentication methods for maximum compatibility:

### Session Authentication (Legacy Compatible)
```http
GET /api/documents/GetDocument?SessionID=abc123&ObjectId=doc_456
Authorization: Session abc123-def456-ghi789
```

### Bearer Token Authentication (Recommended)
```http
GET /api/documents/GetDocument?ObjectId=doc_456
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### Basic Authentication (WebDAV Compatible)
```http
GET /api/documents/GetDocument?ObjectId=doc_456
Authorization: Basic dXNlcm5hbWU6cGFzc3dvcmQ=
```

### Extended Authentication Endpoints

#### Generate Security Token
```http
POST /api/advancedUsers/GenerateSecurityToken
Content-Type: application/json

{
  "Username": "user@kommune.de",
  "Password": "secure_password"
}
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 3600,
  "token_type": "Bearer"
}
```

#### Login with Token
```http
POST /api/advancedUsers/LoginWithToken
Content-Type: application/json

{
  "Token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

---

## 📄 Extended Document Operations

### Import Document by Parent Folder ID
**Endpoint:** `POST /api/basicDocuments/ImportDocumentByParentFolderID`

```http
POST /api/basicDocuments/ImportDocumentByParentFolderID
Authorization: Bearer your-jwt-token
Content-Type: multipart/form-data

{
  "ParentFolderId": "folder_123",
  "File": <binary_data>,
  "FileName": "contract.pdf",
  "Description": "Wichtiger Vertrag"
}
```

**WebDAV Mapping:**
```http
# 1. Resolve folder ID to path
GET /api/folders/GetFolder?FolderId=folder_123
# Response: {"Path": "/Documents/Contracts"}

# 2. Upload to resolved path
PUT /Documents/Contracts/contract.pdf
Content-Type: application/pdf
Authorization: Bearer your-jwt-token
<binary_data>
```

**JavaScript Example:**
```javascript
const client = new OpenYardExtendedClient('https://dms.kommune.de', token);

const result = await client.documents.importByParentFolderId({
  parentFolderId: 'folder_123',
  file: fileBlob,
  fileName: 'contract.pdf',
  description: 'Wichtiger Vertrag'
});

console.log('Document imported:', result.documentId);
```

**Python Example:**
```python
import requests

def import_document_by_folder_id(parent_folder_id, file_path, description=""):
    with open(file_path, 'rb') as f:
        files = {'File': f}
        data = {
            'ParentFolderId': parent_folder_id,
            'FileName': os.path.basename(file_path),
            'Description': description
        }

        response = requests.post(
            'https://dms.kommune.de/api/basicDocuments/ImportDocumentByParentFolderID',
            headers={'Authorization': f'Bearer {token}'},
            files=files,
            data=data
        )

    return response.json()

# Usage
result = import_document_by_folder_id('folder_123', '/path/to/contract.pdf')
```

### Import Document by Parent Folder Path
**Endpoint:** `POST /api/basicDocuments/ImportDocumentByParentFolderPath`

```http
POST /api/basicDocuments/ImportDocumentByParentFolderPath
Authorization: Bearer your-jwt-token
Content-Type: multipart/form-data

{
  "ParentPath": "/Documents/Contracts",
  "File": <binary_data>,
  "FileName": "contract.pdf"
}
```

**Direct WebDAV Mapping:**
```http
PUT /Documents/Contracts/contract.pdf
Content-Type: application/pdf
Authorization: Bearer your-jwt-token
<binary_data>
```

### Rename Document
**Endpoint:** `POST /api/basicDocuments/RenameDocument`

```http
POST /api/basicDocuments/RenameDocument
Authorization: Bearer your-jwt-token
Content-Type: application/json

{
  "ObjectId": "doc_456",
  "NewName": "contract-final.pdf"
}
```

**WebDAV Mapping:**
```http
# 1. Resolve ObjectId to current path
GET /api/documents/GetDocument?ObjectId=doc_456
# Response: {"Path": "/Documents/Contracts/contract.pdf"}

# 2. Execute rename via MOVE
MOVE /Documents/Contracts/contract.pdf
Destination: /Documents/Contracts/contract-final.pdf
Authorization: Bearer your-jwt-token
```

**JavaScript Example:**
```javascript
// Rename document
await client.documents.rename('doc_456', 'contract-final.pdf');

// Batch rename multiple documents
const renameOperations = [
  { objectId: 'doc_456', newName: 'contract-final.pdf' },
  { objectId: 'doc_789', newName: 'invoice-2024.pdf' }
];

for (const op of renameOperations) {
  await client.documents.rename(op.objectId, op.newName);
}
```

### Upload File (Enhanced)
**Endpoint:** `POST /api/basicDocuments/UploadFile`

```http
POST /api/basicDocuments/UploadFile
Authorization: Bearer your-jwt-token
Content-Type: multipart/form-data

{
  "FolderId": "folder_123",
  "File": <binary_data>,
  "OverwriteExisting": true,
  "GeneratePreview": true
}
```

### Document Versioning (Partial Support)

#### Check In and Ignore Changes
**Endpoint:** `POST /api/basicDocuments/CheckInAndIgnoreChanges`

```http
POST /api/basicDocuments/CheckInAndIgnoreChanges
Authorization: Bearer your-jwt-token
Content-Type: application/json

{
  "ObjectId": "doc_456",
  "Comment": "Final version approved"
}
```

**WebDAV DeltaV Mapping:**
```http
CHECKIN /Documents/Contracts/contract.pdf
Content-Type: application/xml
Authorization: Bearer your-jwt-token

<checkin xmlns="DAV:">
  <comment>Final version approved</comment>
</checkin>
```

**Note:** Requires WebDAV server with DeltaV support (RFC 3253)

#### Create New Document Version
**Endpoint:** `POST /api/basicDocuments/CreateNewDocVersion`

```http
POST /api/basicDocuments/CreateNewDocVersion
Authorization: Bearer your-jwt-token
Content-Type: application/json

{
  "ObjectId": "doc_456",
  "File": <binary_data>,
  "VersionComment": "Updated terms and conditions"
}
```

---

## 📁 Extended Folder Operations

### Create Folder by Parent Folder ID
**Endpoint:** `POST /api/basicFolders/CreateFolderByParentFolderID`

```http
POST /api/basicFolders/CreateFolderByParentFolderID
Authorization: Bearer your-jwt-token
Content-Type: application/json

{
  "ParentFolderId": "folder_123",
  "FolderName": "Verträge 2024",
  "Description": "Alle Verträge für das Jahr 2024"
}
```

**WebDAV Mapping:**
```http
# 1. Resolve ParentFolderId to path
GET /api/folders/GetFolder?FolderId=folder_123
# Response: {"Path": "/Documents"}

# 2. Create folder
MKCOL /Documents/Verträge%202024/
Authorization: Bearer your-jwt-token
```

**JavaScript Example:**
```javascript
// Create folder structure for municipal administration
const createMunicipalStructure = async () => {
  const mainFolder = await client.folders.createByParentPath(
    '/Documents',
    'Verwaltung 2024'
  );

  const subFolders = [
    'Verträge',
    'Rechnungen',
    'Beschlüsse',
    'Korrespondenz'
  ];

  for (const folderName of subFolders) {
    await client.folders.createByParentFolderId(
      mainFolder.folderId,
      folderName
    );
  }

  return mainFolder;
};
```

### Create Folder by Parent Folder Path
**Endpoint:** `POST /api/basicFolders/CreateFolderByParentFolderPath`

```http
POST /api/basicFolders/CreateFolderByParentFolderPath
Authorization: Bearer your-jwt-token
Content-Type: application/json

{
  "ParentPath": "/Documents/Verwaltung",
  "FolderName": "Bauanträge 2024"
}
```

**Direct WebDAV Mapping:**
```http
MKCOL /Documents/Verwaltung/Bauanträge%202024/
Authorization: Bearer your-jwt-token
```

### Rename Folder
**Endpoint:** `POST /api/basicFolders/RenameFolder`

```http
POST /api/basicFolders/RenameFolder
Authorization: Bearer your-jwt-token
Content-Type: application/json

{
  "FolderId": "folder_456",
  "NewName": "Archiv 2024"
}
```

### Get Folder Templates (Partial Support)
**Endpoint:** `GET /api/basicFolders/GetFolderTemplates`

```http
GET /api/basicFolders/GetFolderTemplates?CategoryId=municipal
Authorization: Bearer your-jwt-token
```

**Response:**
```json
{
  "templates": [
    {
      "templateId": "municipal_basic",
      "name": "Grundstruktur Verwaltung",
      "folders": [
        "Eingangspost",
        "Ausgangspost",
        "Verträge",
        "Beschlüsse"
      ]
    },
    {
      "templateId": "building_permits",
      "name": "Bauamt Struktur",
      "folders": [
        "Anträge eingegangen",
        "In Bearbeitung",
        "Genehmigt",
        "Abgelehnt"
      ]
    }
  ]
}
```

---

## 🔍 Extended Search Operations

### Search for Index
**Endpoint:** `POST /api/basicCommon/SearchForIndex`

```http
POST /api/basicCommon/SearchForIndex
Authorization: Bearer your-jwt-token
Content-Type: application/json

{
  "SearchTerm": "Bauantrag",
  "IndexFields": ["category", "subject", "reference_number"],
  "DateRange": {
    "from": "2024-01-01",
    "to": "2024-12-31"
  }
}
```

**WebDAV SEARCH Mapping:**
```http
SEARCH /
Content-Type: application/xml
Authorization: Bearer your-jwt-token

<searchrequest xmlns="DAV:">
  <basicsearch>
    <select>
      <prop>
        <displayname/>
        <openyard:category xmlns:openyard="http://openyard.org/ns"/>
        <openyard:subject xmlns:openyard="http://openyard.org/ns"/>
      </prop>
    </select>
    <from>
      <scope>
        <href>/</href>
        <depth>infinity</depth>
      </scope>
    </from>
    <where>
      <contains>Bauantrag</contains>
    </where>
  </basicsearch>
</searchrequest>
```

**JavaScript Example:**
```javascript
// Search for building permits in 2024
const searchBuildingPermits = async () => {
  const results = await client.search.byIndex({
    searchTerm: 'Bauantrag',
    indexFields: ['category', 'subject', 'applicant'],
    dateRange: {
      from: '2024-01-01',
      to: '2024-12-31'
    },
    filters: {
      status: ['approved', 'pending'],
      department: 'Bauamt'
    }
  });

  return results;
};

// Advanced search with facets
const advancedSearch = async (query) => {
  return await client.search.byIndex({
    searchTerm: query,
    facets: {
      byDepartment: true,
      byYear: true,
      byDocumentType: true
    },
    sortBy: 'relevance',
    limit: 50
  });
};
```

### Set Index Data
**Endpoint:** `POST /api/basicCommon/SetIndex`

```http
POST /api/basicCommon/SetIndex
Authorization: Bearer your-jwt-token
Content-Type: application/json

{
  "ObjectId": "doc_456",
  "IndexData": {
    "category": "Bauantrag",
    "subject": "Anbau Einfamilienhaus",
    "applicant": "Max Mustermann",
    "reference_number": "BA-2024-0001",
    "plot_number": "Flst. Nr. 123/4",
    "address": "Musterstraße 123, 12345 Musterstadt"
  }
}
```

**WebDAV PROPPATCH Mapping:**
```http
PROPPATCH /Documents/Bauanträge/BA-2024-0001.pdf
Content-Type: application/xml
Authorization: Bearer your-jwt-token

<propertyupdate xmlns="DAV:">
  <set>
    <prop>
      <openyard:category xmlns:openyard="http://openyard.org/ns">Bauantrag</openyard:category>
      <openyard:subject xmlns:openyard="http://openyard.org/ns">Anbau Einfamilienhaus</openyard:subject>
      <openyard:applicant xmlns:openyard="http://openyard.org/ns">Max Mustermann</openyard:applicant>
      <openyard:reference_number xmlns:openyard="http://openyard.org/ns">BA-2024-0001</openyard:reference_number>
    </prop>
  </set>
</propertyupdate>
```

---

## 🔧 System Operations

### Get API Descriptions
**Endpoint:** `GET /api/ApiExplorer/GetApiDescriptions`

```http
GET /api/ApiExplorer/GetApiDescriptions
Authorization: Bearer your-jwt-token
Accept: application/json
```

**Response:**
```json
{
  "openapi": "3.0.0",
  "info": {
    "title": "OpenYard Cloud API Extended",
    "version": "1.1-extended",
    "description": "Extended API with 76 operations for maximum compatibility"
  },
  "operations": {
    "core": 39,
    "extended": 37,
    "total": 76
  },
  "compatibility": {
    "webdav": "Full support",
    "cs3": "Full support",
    "ocs": "Full support"
  }
}
```

### Get Enumeration as Dictionary
**Endpoint:** `GET /api/basicGeneral/GetEnumerationAsDictionary`

```http
GET /api/basicGeneral/GetEnumerationAsDictionary?Type=DocumentCategories
Authorization: Bearer your-jwt-token
```

**Response:**
```json
{
  "DocumentCategories": {
    "contracts": "Verträge",
    "invoices": "Rechnungen",
    "decisions": "Beschlüsse",
    "correspondence": "Korrespondenz",
    "building_permits": "Bauanträge",
    "protocols": "Protokolle"
  }
}
```

---

## 🌐 Integration Examples

### WebDAV Server Integration

```javascript
// Complete WebDAV server implementation
class OpenYardWebDAVServer {
  constructor(openyardClient) {
    this.client = openyardClient;
    this.pathCache = new Map(); // Cache for path resolution
  }

  async handleRequest(method, path, headers, body) {
    switch (method) {
      case 'GET':
        return this.handleGET(path, headers);
      case 'PUT':
        return this.handlePUT(path, headers, body);
      case 'MOVE':
        return this.handleMOVE(path, headers);
      case 'MKCOL':
        return this.handleMKCOL(path, headers);
      case 'PROPFIND':
        return this.handlePROPFIND(path, headers, body);
      case 'PROPPATCH':
        return this.handlePROPPATCH(path, headers, body);
    }
  }

  async handlePUT(path, headers, content) {
    const parentPath = dirname(path);
    const filename = basename(path);

    // Use ImportDocumentByParentFolderPath for new files
    const result = await this.client.importDocumentByParentFolderPath({
      parentPath: parentPath,
      filename: filename,
      content: content,
      overwriteExisting: true
    });

    return {
      status: 201,
      headers: {
        'ETag': `"${result.etag}"`,
        'Last-Modified': new Date(result.lastModified).toUTCString()
      }
    };
  }

  async handleMOVE(sourcePath, headers) {
    const destPath = headers['destination'];
    const sourceDir = dirname(sourcePath);
    const destDir = dirname(destPath);

    if (sourceDir === destDir) {
      // Rename operation
      const objectId = await this.resolvePathToId(sourcePath);
      await this.client.renameDocument({
        objectId: objectId,
        newName: basename(destPath)
      });
    } else {
      // Move operation
      const objectId = await this.resolvePathToId(sourcePath);
      await this.client.moveObjects([objectId], destDir);
    }

    return { status: 204 };
  }

  async handleMKCOL(path, headers) {
    const parentPath = dirname(path);
    const folderName = basename(path);

    await this.client.createFolderByParentFolderPath({
      parentPath: parentPath,
      folderName: folderName
    });

    return { status: 201 };
  }

  async handlePROPFIND(path, headers, body) {
    const depth = headers['depth'] || 'infinity';
    const folder = await this.client.getFolder({ path: path });

    const response = this.buildPropFindResponse(folder, depth);
    return {
      status: 207,
      headers: { 'Content-Type': 'application/xml' },
      body: response
    };
  }

  async resolvePathToId(path) {
    // Use cache for performance
    if (this.pathCache.has(path)) {
      return this.pathCache.get(path);
    }

    const result = await this.client.getDocumentByPath({ path: path });
    this.pathCache.set(path, result.objectId);
    return result.objectId;
  }
}
```

### CS3 Storage Provider

```go
// CS3 Storage Provider Implementation
package openyard

import (
    "context"
    "path/filepath"
    provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
    "github.com/cs3org/reva/pkg/storage"
)

type OpenYardStorageProvider struct {
    client *OpenYardExtendedClient
}

func (p *OpenYardStorageProvider) CreateContainer(ctx context.Context, ref *provider.Reference) error {
    parentPath := filepath.Dir(ref.Path)
    folderName := filepath.Base(ref.Path)

    return p.client.CreateFolderByParentFolderPath(&CreateFolderRequest{
        ParentPath: parentPath,
        FolderName: folderName,
    })
}

func (p *OpenYardStorageProvider) InitiateFileUpload(ctx context.Context, ref *provider.Reference, size uint64) (*provider.UploadProtocol, error) {
    parentPath := filepath.Dir(ref.Path)
    filename := filepath.Base(ref.Path)

    uploadInfo, err := p.client.PrepareFileUpload(&UploadRequest{
        ParentPath: parentPath,
        Filename:   filename,
        Size:       size,
    })

    return &provider.UploadProtocol{
        Protocol: "simple",
        UploadEndpoint: uploadInfo.UploadURL,
        ExpirationTime: uploadInfo.ExpiresAt,
    }, err
}

func (p *OpenYardStorageProvider) Move(ctx context.Context, oldRef, newRef *provider.Reference) error {
    oldDir := filepath.Dir(oldRef.Path)
    newDir := filepath.Dir(newRef.Path)

    if oldDir == newDir {
        // Rename operation
        return p.client.RenameDocument(&RenameRequest{
            ObjectId: oldRef.ResourceId.OpaqueId,
            NewName:  filepath.Base(newRef.Path),
        })
    } else {
        // Move operation
        return p.client.MoveObjects(&MoveRequest{
            ObjectIds:       []string{oldRef.ResourceId.OpaqueId},
            DestinationPath: newDir,
        })
    }
}

func (p *OpenYardStorageProvider) ListContainer(ctx context.Context, ref *provider.Reference) ([]*provider.ResourceInfo, error) {
    folder, err := p.client.GetFolderByPath(&GetFolderRequest{
        Path: ref.Path,
    })

    if err != nil {
        return nil, err
    }

    var resources []*provider.ResourceInfo

    for _, item := range folder.Contents {
        resource := &provider.ResourceInfo{
            Id: &provider.ResourceId{
                OpaqueId: item.ObjectId,
            },
            Path: item.Path,
            Type: provider.ResourceType_RESOURCE_TYPE_FILE,
            Size: item.Size,
            Mtime: timestamppb.New(item.ModifiedAt),
        }

        if item.Type == "folder" {
            resource.Type = provider.ResourceType_RESOURCE_TYPE_CONTAINER
        }

        resources = append(resources, resource)
    }

    return resources, nil
}
```

### Municipal Administration Integration

```python
# Complete municipal administration system integration
import asyncio
from openyard_extended import OpenYardExtendedClient

class MunicipalDMSIntegration:
    def __init__(self, base_url, token):
        self.client = OpenYardExtendedClient(base_url, token)

    async def setup_municipal_structure(self):
        """Create standard municipal folder structure"""
        # Create main departments
        departments = [
            ("Bürgermeisteramt", ["Protokolle", "Korrespondenz", "Verträge"]),
            ("Bauamt", ["Bauanträge", "Genehmigungen", "Abnahmen"]),
            ("Kämmerei", ["Haushaltsplan", "Rechnungen", "Belege"]),
            ("Ordnungsamt", ["Bescheide", "Meldungen", "Protokolle"]),
            ("Standesamt", ["Geburten", "Trauungen", "Sterbefälle"])
        ]

        for dept_name, sub_folders in departments:
            # Create department folder
            dept_folder = await self.client.folders.create_by_parent_path(
                parent_path="/Verwaltung",
                folder_name=dept_name
            )

            # Create sub-folders
            for sub_folder in sub_folders:
                await self.client.folders.create_by_parent_folder_id(
                    parent_folder_id=dept_folder['folderId'],
                    folder_name=sub_folder
                )

    async def import_building_permit_documents(self, permit_data):
        """Import building permit with all related documents"""
        # Create permit-specific folder
        permit_folder_name = f"BA-{permit_data['year']}-{permit_data['number']:04d}"
        permit_folder = await self.client.folders.create_by_parent_path(
            parent_path="/Verwaltung/Bauamt/Bauanträge",
            folder_name=permit_folder_name
        )

        # Import main application
        main_doc = await self.client.documents.import_by_parent_folder_id(
            parent_folder_id=permit_folder['folderId'],
            file_path=permit_data['application_pdf'],
            file_name=f"{permit_folder_name}_Antrag.pdf"
        )

        # Set building permit metadata
        await self.client.metadata.set_index(
            object_id=main_doc['documentId'],
            index_data={
                'category': 'Bauantrag',
                'subject': permit_data['subject'],
                'applicant': permit_data['applicant'],
                'reference_number': permit_folder_name,
                'plot_number': permit_data['plot_number'],
                'address': permit_data['address'],
                'application_date': permit_data['application_date'],
                'status': 'eingegangen'
            }
        )

        # Import supporting documents
        supporting_docs = [
            'plans', 'calculations', 'photos', 'certificates'
        ]

        for doc_type in supporting_docs:
            if doc_type in permit_data['documents']:
                sub_folder = await self.client.folders.create_by_parent_folder_id(
                    parent_folder_id=permit_folder['folderId'],
                    folder_name=doc_type.title()
                )

                for doc_file in permit_data['documents'][doc_type]:
                    await self.client.documents.import_by_parent_folder_id(
                        parent_folder_id=sub_folder['folderId'],
                        file_path=doc_file['path'],
                        file_name=doc_file['name']
                    )

        return permit_folder

    async def generate_building_permit_report(self, year):
        """Generate building permit statistics for the year"""
        # Search for all building permits in the year
        results = await self.client.search.by_index(
            search_term="Bauantrag",
            index_fields=['category', 'reference_number', 'status'],
            date_range={'from': f"{year}-01-01", 'to': f"{year}-12-31"}
        )

        # Group by status
        status_counts = {}
        for doc in results['documents']:
            status = doc['metadata']['status']
            status_counts[status] = status_counts.get(status, 0) + 1

        return {
            'year': year,
            'total_applications': len(results['documents']),
            'status_breakdown': status_counts,
            'applications': results['documents']
        }

# Usage example
async def main():
    dms = MunicipalDMSIntegration(
        base_url="https://dms.musterstadt.de",
        token="your-jwt-token"
    )

    # Setup municipal structure
    await dms.setup_municipal_structure()

    # Import building permit
    permit_data = {
        'year': 2024,
        'number': 1,
        'subject': 'Anbau Einfamilienhaus',
        'applicant': 'Max Mustermann',
        'plot_number': 'Flst. Nr. 123/4',
        'address': 'Musterstraße 123, 12345 Musterstadt',
        'application_date': '2024-01-15',
        'application_pdf': '/tmp/bauantrag.pdf',
        'documents': {
            'plans': [
                {'path': '/tmp/grundriss.pdf', 'name': 'Grundriss.pdf'},
                {'path': '/tmp/ansicht.pdf', 'name': 'Ansicht.pdf'}
            ],
            'photos': [
                {'path': '/tmp/foto1.jpg', 'name': 'Bestandsfoto.jpg'}
            ]
        }
    }

    permit_folder = await dms.import_building_permit_documents(permit_data)
    print(f"Building permit imported: {permit_folder['folderId']}")

    # Generate annual report
    report = await dms.generate_building_permit_report(2024)
    print(f"2024 Building Permits: {report['total_applications']} total")

if __name__ == "__main__":
    asyncio.run(main())
```

---

## ⚡ Performance Optimization

### Caching Strategy

```yaml
# Extended API Caching Configuration
caching:
  path_resolution:
    ttl: 300s
    max_size: 10000
    backend: redis

  permissions:
    ttl: 60s
    max_size: 5000

  metadata:
    ttl: 120s
    max_size: 20000

  folders:
    ttl: 180s
    max_size: 15000
```

### Batch Operations

```javascript
// Batch import for better performance
const batchImportDocuments = async (documents) => {
  const BATCH_SIZE = 10;
  const results = [];

  for (let i = 0; i < documents.length; i += BATCH_SIZE) {
    const batch = documents.slice(i, i + BATCH_SIZE);

    const batchPromises = batch.map(doc =>
      client.documents.importByParentFolderId({
        parentFolderId: doc.folderId,
        file: doc.file,
        fileName: doc.fileName
      })
    );

    const batchResults = await Promise.all(batchPromises);
    results.push(...batchResults);

    // Small delay to prevent overwhelming the server
    await new Promise(resolve => setTimeout(resolve, 100));
  }

  return results;
};
```

---

## 🚨 Error Handling

### Extended Error Codes

```json
{
  "error_codes": {
    "FOLDER_ID_NOT_FOUND": {
      "code": 404,
      "message": "Folder with specified ID not found",
      "webdav_mapping": 404
    },
    "INVALID_FOLDER_PATH": {
      "code": 400,
      "message": "Invalid folder path specified",
      "webdav_mapping": 409
    },
    "UPLOAD_SIZE_EXCEEDED": {
      "code": 413,
      "message": "File size exceeds maximum allowed",
      "webdav_mapping": 413
    },
    "VERSIONING_NOT_SUPPORTED": {
      "code": 501,
      "message": "Document versioning not available",
      "webdav_mapping": 501
    }
  }
}
```

### Error Handling Examples

```javascript
// Comprehensive error handling
const robustDocumentImport = async (parentFolderId, file, fileName) => {
  try {
    return await client.documents.importByParentFolderId({
      parentFolderId,
      file,
      fileName
    });
  } catch (error) {
    switch (error.code) {
      case 'FOLDER_ID_NOT_FOUND':
        // Try to create parent folder structure
        const folders = parentFolderId.split('/').filter(Boolean);
        let currentPath = '';

        for (const folderName of folders) {
          currentPath += `/${folderName}`;
          try {
            await client.folders.createByParentFolderPath({
              parentPath: dirname(currentPath),
              folderName: basename(currentPath)
            });
          } catch (createError) {
            if (createError.code !== 'FOLDER_ALREADY_EXISTS') {
              throw createError;
            }
          }
        }

        // Retry import
        return await client.documents.importByParentFolderId({
          parentFolderId,
          file,
          fileName
        });

      case 'UPLOAD_SIZE_EXCEEDED':
        // Implement chunked upload for large files
        return await uploadLargeFile(parentFolderId, file, fileName);

      case 'INVALID_FILENAME':
        // Sanitize filename and retry
        const sanitizedName = sanitizeFileName(fileName);
        return await client.documents.importByParentFolderId({
          parentFolderId,
          file,
          fileName: sanitizedName
        });

      default:
        throw error;
    }
  }
};
```

---

## 🔧 Configuration Examples

### Docker Compose Extended Setup

```yaml
version: '3.8'
services:
  openyard-extended:
    image: openyard/cloud-api:1.1-extended
    environment:
      # Core settings
      - OPENYARD_VERSION=1.1-extended
      - TOTAL_OPERATIONS=76
      - EXTENDED_MAPPINGS=true

      # Protocol support
      - WEBDAV_ENABLED=true
      - CS3_ENABLED=true
      - OCS_ENABLED=true

      # Performance settings
      - CACHE_BACKEND=redis
      - BATCH_SIZE=10
      - MAX_UPLOAD_SIZE=100MB

      # Extended features
      - VERSIONING_ENABLED=true
      - SEARCH_INDEXING=true
      - METADATA_EXTRACTION=true

    ports:
      - "8080:8080"
      - "8443:8443"  # HTTPS

    volumes:
      - ./config/extended.yml:/config/api.yml
      - ./logs:/var/log/openyard
      - ./data:/var/lib/openyard

    depends_on:
      - redis
      - postgresql

    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  redis:
    image: redis:alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  postgresql:
    image: postgres:13
    environment:
      POSTGRES_DB: openyard
      POSTGRES_USER: openyard
      POSTGRES_PASSWORD: secure_password
    volumes:
      - postgres_data:/var/lib/postgresql/data

volumes:
  redis_data:
  postgres_data:
```

---

## 📊 Monitoring and Analytics

### Extended API Metrics

```javascript
// Monitoring extended operations
const monitoringConfig = {
  metrics: {
    // Core operations
    'documents.get': { threshold: 100, unit: 'ms' },
    'documents.set': { threshold: 500, unit: 'ms' },

    // Extended operations
    'documents.import_by_folder_id': { threshold: 800, unit: 'ms' },
    'documents.rename': { threshold: 200, unit: 'ms' },
    'folders.create_by_path': { threshold: 300, unit: 'ms' },
    'search.by_index': { threshold: 2000, unit: 'ms' },

    // Mapping operations
    'webdav.path_resolution': { threshold: 50, unit: 'ms' },
    'cs3.protocol_translation': { threshold: 30, unit: 'ms' }
  },

  alerts: {
    error_rate: { threshold: 1, unit: '%' },
    cache_miss_rate: { threshold: 10, unit: '%' },
    extended_operation_usage: { threshold: 80, unit: '%' }
  }
};
```

---

**OpenYard 1.1 Extended** provides comprehensive compatibility while maintaining cloud-native principles. The extensive mapping layers ensure smooth integration with existing systems while enabling modern cloud deployments.